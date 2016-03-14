/*
	This file is part of the card package.
	This specifically deals with processing charges (and refunding charges).
	These functions are broken out into a separate file for organizational purposes.
*/

package card

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/coreymgilmore/timestamps"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/refund"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	"memcacheutils"
	"output"
	"sessionutils"
)

//CHARGE A CARD
func Charge(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreId := r.FormValue("datastoreId")
	customerName := r.FormValue("customerName")
	amount := r.FormValue("amount")
	invoice := r.FormValue("invoice")
	poNum := r.FormValue("po")

	//validation
	if len(datastoreId) == 0 {
		output.Error(ErrMissingInput, "A customer ID should have been submitted automatically but was not. Please contact an administrator.", w)
		return
	}
	if len(amount) == 0 {
		output.Error(ErrMissingInput, "No amount was provided. You cannot charge a card nothing!", w)
		return
	}

	//get amount as cents
	amountCents, err := getAmountAsIntCents(amount)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w)
		return
	}

	//check if amount is greater than the minimum charge
	//min charge may be greater than 0 because of transactions costs
	//for example, stripe takes 30 cents...it does not make sense to charge a card for < 30 cents
	if amountCents < minCharge {
		output.Error(ErrChargeAmountTooLow, "You must charge at least "+strconv.FormatInt(minCharge, 10)+" cents.", w)
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
		output.Error(err, "An error occured while looking up the customer's Stripe information.", w)
		return
	}

	//make sure customer name matches
	//just another catch in case of strange errors and mismatched data
	if customerName != custData.CustomerName {
		output.Error(err, "The customer name did not match the data for the customer ID. Please log out and try again.", w)
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
			errorMsg = "Charging this card timed out.  The charge may have succeeded anyway. Please check the Report to see if this charge was successful."
			break
		case *stripe.Error:
			stripeErr := err.(*stripe.Error)
			errorMsg = stripeErr.Msg
		}

		output.Error(ErrStripe, errorMsg, w)
		return
	}

	//charge successful
	//save charge to memcache
	//less data to get from stripe if receipt is needed
	memcacheutils.Save(c, chg.ID, chg)

	//save count of card types
	//done in goroutine to stop blocking returning data to user
	go saveChargeDetails(c, chg)

	//return to client
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

//REFUND A CHARGE
func Refund(w http.ResponseWriter, r *http.Request) {
	//get form values
	chargeId := r.FormValue("chargeId")
	amount := r.FormValue("amount")
	reason := r.FormValue("reason")

	//make sure inputs were given
	if len(chargeId) == 0 {
		output.Error(ErrMissingInput, "A charge ID was not provided. This is a serious error. Please contact an administrator.", w)
		return
	}
	if len(amount) == 0 {
		output.Error(ErrMissingInput, "No amount was given to refund.", w)
		return
	}

	//convert refund amount to cents
	//stripe requires cents
	amountCents, err := getAmountAsIntCents(amount)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w)
		return
	}

	//get username of logged in user
	//for tracking who processed this refund
	session := sessionutils.Get(r)
	username := session.Values["username"].(string)

	//build refund
	params := &stripe.RefundParams{
		Charge: chargeId,
		Amount: amountCents,
	}

	//add metadata to refund
	//same field name as when creating a charge
	params.AddMeta("charged_by", username)

	//get reason code for refund
	if reason == "duplicate" {
		params.Reason = refund.RefundDuplicate
	} else if reason == "requested_by_customer" {
		params.Reason = refund.RefundRequestedByCustomer
	}

	//init stripe
	c := appengine.NewContext(r)
	sc := createAppengineStripeClient(c)

	//create refund with stripe
	_, err = sc.Refunds.New(params)
	if err != nil {
		stripeErr := err.(*stripe.Error)
		stripeErrMsg := stripeErr.Msg
		output.Error(ErrStripe, stripeErrMsg, w)
		return
	}

	//done
	output.Success("refund-done", nil, w)
	return
}

//SAVE COUNT OF CARD CHARGED AND CARD TYPES CHARGED
//this is used to keep track of how many charges are processed and for what card types
//use this info to negotiate better rates with Stripe (not saying Stripe isn't honest, but this gives you accurate data)
//this just increments some counters in a transaction
//this should be run in a goroutine so that the parent http calls response is sent back to user faster
func saveChargeDetails(c context.Context, chg *stripe.Charge) {
	//FORMAT OF DATA IN DATASTORE
	//total is the total number for charges performed
	//each card type is the total per card type
	//list of card types from https://github.com/stripe/stripe-go/blob/6e49b4ff8c8b6fd2b32499ccad12f3e2fc302a87/card.go
	type cardCounts struct {
		Total int

		Unknown         int
		Visa            int
		AmericanExpress int
		MasterCard      int
		Discover        int
		JCB             int
		DinersClub      int
	}

	//DATASTORE KIND TO SAVE DETAILS UNDER
	//separate kind that holds just this data
	const kind = "charge-details"

	//KEY NAME
	//so we don't have to keep track of a random integer
	//this replaces the IntID
	const keyName = "card-count"

	//GET CARD BRAND FROM CHARGE
	brand := string(chg.Source.Card.Brand)

	//GET COMPLETE DATASTORE KEY TO LOOKUP AND UPDATE
	//this is the key of the entity that store the card count data
	key := datastore.NewKey(c, kind, keyName, 0, nil)

	//TRANSACTION
	err := datastore.RunInTransaction(c, func(c context.Context) error {
		//LOOK UP DATA FROM DATASTORE
		r := new(cardCounts)
		err := datastore.Get(c, key, r)
		if err != nil && err != datastore.ErrNoSuchEntity {
			log.Errorf(c, "%v", "Error looking up card brand count.")
			log.Errorf(c, "%v", err)
		}

		//INCREMENT COUNTER FOR TOTAL
		r.Total++

		//INCREMENT COUNTER FOR CARD BRAND
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
			log.Debugf(c, "%v", "Unknown card type:", brand)
		}

		//SAVE DATA BACK TO DB
		//perform "update"
		_, err = datastore.Put(c, key, r)
		if err != nil {
			log.Errorf(c, "%v", "Error saving card brand count.")
			log.Errorf(c, "%v", err)
		}

		//done
		//returns nill if everything is ok and update was performed
		return err
	}, nil)
	if err != nil {
		log.Errorf(c, "%v", "Error during card brand count transaction.")
		log.Errorf(c, "%v", err)
	}

	//done
	log.Debugf(c, brand)
	return
}
