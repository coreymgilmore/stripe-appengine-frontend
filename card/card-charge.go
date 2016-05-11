/*
File card-charge.go implements functionality to charge a card.
*/

package card

import (
	"github.com/coreymgilmore/timestamps"
	"github.com/stripe/stripe-go"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"memcacheutils"
	"net/http"
	"net/url"
	"output"
	"sessionutils"
	"strconv"
	"time"
)

//Charge charges a credit card
func Charge(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreId := r.FormValue("datastoreId")
	customerName := r.FormValue("customerName")
	amount := r.FormValue("amount")
	invoice := r.FormValue("invoice")
	poNum := r.FormValue("po")

	//validation
	if len(datastoreId) == 0 {
		output.Error(ErrMissingInput, "A customer ID should have been submitted automatically but was not. Please contact an administrator.", w, r)
		return
	}
	if len(amount) == 0 {
		output.Error(ErrMissingInput, "No amount was provided. You cannot charge a card nothing!", w, r)
		return
	}

	//get amount as cents
	amountCents, err := getAmountAsIntCents(amount)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w, r)
		return
	}

	//check if amount is greater than the minimum charge
	//min charge may be greater than 0 because of transactions costs
	//for example, stripe takes 30 cents...it does not make sense to charge a card for < 30 cents
	if amountCents < minCharge {
		output.Error(ErrChargeAmountTooLow, "You must charge at least "+strconv.FormatInt(minCharge, 10)+" cents.", w, r)
		return
	}

	//create context
	//need to adjust deadline in case stripe takes longer than 5 seconds
	//default timeout for a urlfetch is 5 seconds
	//sometimes charging a card through stripe api takes longer
	//calls seems to take roughly 2 seconds normally with a few near 5 seconds (old deadline)
	//the call might still complete via stripe but appengine will return to the gui that it failed
	//10 secodns is a bit over generous but covers even really strange senarios
	c := appengine.NewContext(r)
	c, _ = context.WithTimeout(c, 10*time.Second)

	//look up stripe customer id from datastore
	datastoreIdInt, _ := strconv.ParseInt(datastoreId, 10, 64)
	custData, err := findByDatastoreId(c, datastoreIdInt)
	if err != nil {
		output.Error(err, "An error occured while looking up the customer's Stripe information.", w, r)
		return
	}

	//make sure customer name matches
	//just another catch in case of strange errors and mismatched data
	if customerName != custData.CustomerName {
		output.Error(err, "The customer name did not match the data for the customer ID. Please log out and try again.", w, r)
		return
	}

	//get username of logged in user
	//used for tracking who processed a charge
	//for audits and reports
	session := sessionutils.Get(r)
	username := session.Values["username"].(string)

	//init stripe
	sc := createAppengineStripeClient(c)

	//build charge object
	chargeParams := &stripe.ChargeParams{
		Customer:  custData.StripeCustomerToken,
		Amount:    amountCents,
		Currency:  currency,
		Desc:      "Charge for invoice: " + invoice + ", purchase order: " + poNum + ".",
		Statement: formatStatementDescriptor(),
	}

	//add metadata to charge
	//used for reports and receipts
	chargeParams.AddMeta("customer_name", customerName)
	chargeParams.AddMeta("datastore_id", datastoreId)
	chargeParams.AddMeta("customer_id", custData.CustomerId)
	chargeParams.AddMeta("invoice_num", invoice)
	chargeParams.AddMeta("po_num", poNum)
	chargeParams.AddMeta("charged_by", username)

	//process the charge
	chg, err := sc.Charges.New(chargeParams)

	//handle errors
	//*url.Error can be thrown if urlfetch reaches timeout (request took too long to complete)
	//*stripe.Error is a error with the stripe api and should return a human readable error message
	if err != nil {
		errorMsg := ""

		switch err.(type) {
		default:
			errorMsg = "There was an error processing this charge. Please check the Report to see if this charge was successful."
			break
		case *url.Error:
			errorMsg = "Charging this card timed out. The charge may have succeeded anyway. Please check the Report to see if this charge was successful."
			break
		case *stripe.Error:
			stripeErr := err.(*stripe.Error)
			errorMsg = stripeErr.Msg
		}

		output.Error(ErrStripe, errorMsg, w, r)
		return
	}

	//charge successful
	//save charge to memcache
	//less data to get from stripe if receipt is needed
	//errors are ignores since if we can't save this data to memcache we can always get it from the datastore/stripe
	memcacheutils.Save(c, chg.ID, chg)

	//save count of card types
	//used for negotiating rates with Stripe and just extra info
	saveChargeDetails(c, chg)

	//build struct to output a success message to the client
	out := chargeSuccessful{
		CustomerName:   customerName,
		Cardholder:     custData.Cardholder,
		CardExpiration: custData.CardExpiration,
		CardLast4:      custData.CardLast4,
		Amount:         amount,
		Invoice:        invoice,
		Po:             poNum,
		Datetime:       timestamps.ISO8601(),
		ChargeId:       chg.ID,
	}
	output.Success("cardCharged", out, w)
	return
}

//saveChargeDetails increments the number of times each type of card is charged and saves this data to the datastore
//use this info to negotiate better rates with Stripe (not saying Stripe isn't honest, but this gives you accurate data)
func saveChargeDetails(c context.Context, chg *stripe.Charge) {
	//format of data in datastore
	//total is the total number for charges performed
	//each card type is the total number of charges for that per card type
	//list of card types from https://github.com/stripe/stripe-go/blob/6e49b4ff8c8b6fd2b32499ccad12f3e2fc302a87/card.go
	type cardCounts struct {
		Total           int
		Unknown         int
		Visa            int
		AmericanExpress int
		MasterCard      int
		Discover        int
		JCB             int
		DinersClub      int
	}

	//datastore kind to save details under
	//separate kind that holds just this data
	const kind = "chargeDetails"

	//key name
	//so we don't have to keep track of a random integer
	//this replaces the IntID
	const keyName = "card-count"

	//get card brand from charge
	brand := string(chg.Source.Card.Brand)

	//get complete datastore key to lookup and update
	//this is the key of the entity that store the card count data
	key := datastore.NewKey(c, kind, keyName, 0, nil)

	//transaction
	err := datastore.RunInTransaction(c, func(c context.Context) error {
		//look up data from datastore
		r := new(cardCounts)
		err := datastore.Get(c, key, r)
		if err != nil && err != datastore.ErrNoSuchEntity {
			log.Errorf(c, "%v", "Error looking up card brand count.", err)
		}

		//increment counter for total
		r.Total++

		//increment counter for card brand
		switch brand {
		case "Visa":
			r.Visa++
		case "American Express":
			r.AmericanExpress++
		case "MasterCard":
			r.MasterCard++
		case "Discover":
			r.Discover++
		case "JCB":
			r.JCB++
		case "Diners Club":
			r.DinersClub++
		default:
			r.Unknown++
			log.Warningf(c, "%v", "%v", "Unknown card type:", brand)
		}

		//save data back to db
		//perform "update"
		_, err = datastore.Put(c, key, r)
		if err != nil {
			log.Errorf(c, "%v", "Error saving card brand count.", err)
		}

		//done
		//returns nill if everything is ok and update was performed
		return err
	}, nil)
	if err != nil {
		log.Errorf(c, "%v", "Error during card brand count transaction.", err)
	}

	//done
	log.Infof(c, "%v", "Card Brand:", brand)
	return
}
