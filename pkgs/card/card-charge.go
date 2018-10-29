package card

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/appsettings"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/company"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/timestamps"
	"github.com/stripe/stripe-go"
)

//ManualCharge processes a charge on a credit card
//this is used when a user clicks the charge button in the gui
func ManualCharge(w http.ResponseWriter, r *http.Request) {
	//get inputs
	datastoreID, _ := strconv.ParseInt(r.FormValue("datastoreId"), 10, 64) //id from datastore
	amount := r.FormValue("amount")                                        //in dollars
	invoice := r.FormValue("invoice")
	poNum := r.FormValue("po")
	chargeAndRemove, _ := strconv.ParseBool(r.FormValue("chargeAndRemove")) //true if card should be removed after charging

	//validation
	if datastoreID == 0 {
		output.Error(errMissingInput, "A customer ID should have been submitted automatically but was not. Please contact an administrator.", w)
		return
	}
	if len(amount) == 0 {
		output.Error(errMissingInput, "No amount was provided. You cannot charge a card nothing!", w)
		return
	}

	//get username of logged in user
	//we record this data so we can see who processed a charge in the reports
	username := sessionutils.GetUsername(r)

	//get amount as cents
	//stripe requires the amount as a whole number
	amountCents, err := getAmountAsIntCents(amount)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w)
		return
	}

	//create context
	//need to adjust deadline in case stripe takes longer than 5 seconds
	c := r.Context()
	c, cancelFunc := context.WithTimeout(c, 10*time.Second)
	defer cancelFunc()

	//look up stripe customer id from datastore
	custData, err := findByDatastoreID(c, datastoreID)
	if err != nil {
		output.Error(err, "An error occured while looking up the customer's Stripe information.", w)
		return
	}

	//get statement descriptor from company info
	companyInfo, err := company.Get(r)
	if err != nil {
		output.Error(err, "Could not get statement descriptor from company info.", w)
		return
	} else if len(companyInfo.StatementDescriptor) == 0 {
		output.Error(nil, "Your company does not have a statement descriptor set.  Please ask an admin to set one.", w)
		return
	}

	out, errMsg, err := processCharge(c, amountCents, invoice, poNum, companyInfo, custData, username, "", "")
	if err != nil {
		output.Error(err, errMsg, w)
		return
	}

	//charge successful
	//check if we need to remove this card
	//remove it if necessary
	if chargeAndRemove {
		err := Remove(strconv.FormatInt(datastoreID, 10), r)
		if err != nil {
			log.Println("Error removing card after charge.", err)
		}
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

	//connect to datastore
	dsClient, err := datastoreutils.Connect(c)
	if err != nil {
		return
	}

	//get the key we are saving to
	key := datastore.NameKey(kind, keyName, nil)

	//transaction
	_, err = dsClient.RunInTransaction(c, func(tx *datastore.Transaction) error {
		//look up data from datastore
		var r cardCounts
		err := tx.Get(key, &r)
		if err != nil && err != datastore.ErrNoSuchEntity {
			log.Println("card.saveChargeDetails", "Error looking up card brand count.", err)
			return err
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
			log.Println("card.saveChargeDetails", "Unknown card type:", brand)
			return err
		}

		//save data back to db
		//perform "update"
		_, err = tx.Put(key, r)
		if err != nil {
			log.Println("card.saveChargeDetails", "Error saving card brand count.", err)
			return err
		}

		//done
		//returns nill if everything is ok and update was performed
		return err
	})
	if err != nil {
		log.Println("card.saveChargeDetails", "Error during card brand count transaction.", err)
		return
	}

	//done
	return
}

//AutoCharge processes a charge on a credit card automatically
//this is used to charge a card without using the gui
func AutoCharge(w http.ResponseWriter, r *http.Request) {
	//get inputs
	customerID := r.FormValue("customer_id") //the id in the CRM system, not the datastore ID since we dont store that off of appengine
	amount := r.FormValue("amount")          //in cents
	invoice := r.FormValue("invoice")
	poNum := r.FormValue("po")
	apiKey := r.FormValue("api_key")
	autoCharge, _ := strconv.ParseBool(r.FormValue("auto_charge")) //true if we should actually charge the card, false for testing
	referrer := r.FormValue("auto_charge_referrer")                //the name or other identifier for the app making this request to charge the card
	reason := r.FormValue("auto_charge_reason")                    //the action or other identifier within the app making this request (if the referrer has many actions to charge a card, this lets you figure out which action charged the card)

	//validation
	if customerID == "" {
		output.Error(errMissingInput, "A customer ID should have been submitted.", w)
		return
	}
	if len(amount) == 0 {
		output.Error(errMissingInput, "No amount was provided.", w)
		return
	}
	if autoCharge == false {
		output.Error(errMissingInput, "The 'auto_charge' value was not provided. This is required when trying to automatically process a charge.", w)
		return
	}
	if len(referrer) == 0 {
		output.Error(errMissingInput, "There was no 'referrer' given.  This should be the app that made this auto-charge request.  This is used for logging.", w)
		return
	}
	if len(apiKey) == 0 {
		output.Error(errMissingInput, "There was no api given. This must be given in the 'api_key' field to authenticate this request.", w)
		return
	}

	//verify api key
	settings, err := appsettings.Get(r)
	if err != nil {
		output.Error(err, "Could not get app settings to verify api key.", w)
		return
	}
	if settings.APIKey != apiKey {
		output.Error(errMissingInput, "The api key provided in the request is not correct.", w)
		return
	}

	//convert amount to uint
	amountCents, err := strconv.ParseUint(amount, 10, 64)
	if err != nil {
		output.Error(err, "Could not convert amount to integer.", w)
		return
	}

	//create context
	//need to adjust deadline in case stripe takes longer than 5 seconds
	c := r.Context()
	c, cancelFunc := context.WithTimeout(c, 10*time.Second)
	defer cancelFunc()

	//look up stripe customer id from datastore
	custData, err := FindByCustomerID(c, customerID)
	if err != nil {
		output.Error(err, "An error occured while looking up the customer's Stripe information.", w)
		return
	}

	//get statement descriptor from company info
	companyInfo, err := company.Get(r)
	if err != nil {
		output.Error(err, "Could not get statement descriptor from company info.", w)
		return
	} else if len(companyInfo.StatementDescriptor) == 0 {
		output.Error(nil, "Your company does not have a statement descriptor set.  Please ask an admin to set one.", w)
		return
	}

	out, errMsg, err := processCharge(c, amountCents, invoice, poNum, companyInfo, custData, "api", referrer, reason)
	if err != nil {
		output.Error(err, errMsg, w)
		return
	}

	output.Success("cardCharged", out, w)
	return
}

//isBelowMinCharge checks if an amount to charge is too low and returns an error message
//min charge may be greater than 0 because of transactions costs
//for example, stripe takes 30 cents...it does not make sense to charge a card for < 30 cents
func isBelowMinCharge(amount uint64) (string, error) {
	if amount < minCharge {
		return "You must charge at least " + strconv.FormatInt(minCharge, 10) + " cents.", errChargeAmountTooLow
	}

	return "", nil
}

//processCharge peforms most of the actions required to actually charge a card
//this func removes a lot of retyping between ManualCharge and AutoCharge
//c: used for stripe client
//amountCents: amount to charge
//invoiceNum & poNum: details about the order charge is for
//companyInfo: statement descriptor
//custData: data about the customer who this charge is for
//user: either the logged in user or "api" when charged automatically
//referrer & reason: only given when charge is automatically processed
func processCharge(c context.Context, amountCents uint64, invoiceNum, poNum string, companyInfo company.Info, custData CustomerDatastore, user, referrer, reason string) (out chargeSuccessful, errMsg string, err error) {
	//get stripe client
	sc := createAppengineStripeClient(c)

	//check if invoice or po number are blank
	//so that the description on stripe's dashboard makes sense if values are missing
	if len(invoiceNum) == 0 {
		invoiceNum = "*not provided*"
	}
	if len(poNum) == 0 {
		poNum = "*not provided*"
	}

	//build charge object
	chargeParams := &stripe.ChargeParams{
		Customer:            stripe.String(custData.StripeCustomerToken),
		Amount:              stripe.Int64(int64(amountCents)),
		Currency:            stripe.String(currency),
		Description:         stripe.String("Charge for invoice: " + invoiceNum + ", purchase order: " + poNum + "."),
		StatementDescriptor: stripe.String(companyInfo.StatementDescriptor),
	}

	//add metadata
	chargeParams.AddMetadata("customer_name", custData.CustomerName)
	chargeParams.AddMetadata("customer_id", custData.CustomerID)
	chargeParams.AddMetadata("invoice_num", invoiceNum)
	chargeParams.AddMetadata("po_num", poNum)
	chargeParams.AddMetadata("processed_by", user)

	if user == "api" {
		chargeParams.AddMetadata("auto_charge_referrer", referrer)
		chargeParams.AddMetadata("auto_charge_reason", reason)
	}

	//process the charge
	chg, err := sc.Charges.New(chargeParams)

	//handle errors
	//*url.Error can be thrown if urlfetch reaches timeout (request took too long to complete)
	//*stripe.Error is a error with the stripe api and should return a human readable error message
	if err != nil {
		switch err.(type) {
		default:
			errMsg = "There was an error processing this charge. Please check the Report to see if this charge was successful."
			return
		case *url.Error:
			errMsg = "Charging this card timed out. The charge may have succeeded anyway. Please check the Report to see if this charge was successful."
			return
		case *stripe.Error:
			stripeErr := err.(*stripe.Error)
			errMsg = stripeErr.Msg
			return
		}
	}

	//save count of card types
	saveChargeDetails(c, chg)

	//build struct to output a success message to the client
	out = chargeSuccessful{
		Cardholder:     custData.Cardholder,
		CardExpiration: custData.CardExpiration,
		CardLast4:      custData.CardLast4,
		Amount:         strconv.Itoa(int(amountCents) / 100),
		Invoice:        invoiceNum,
		Po:             poNum,
		Datetime:       timestamps.ISO8601(),
		ChargeID:       chg.ID,
	}
	return
}
