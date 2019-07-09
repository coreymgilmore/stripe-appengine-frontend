package card

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/appsettings"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/company"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sqliteutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/timestamps"
	"github.com/stripe/stripe-go"
)

//processChargeInputs is the data used in the processCharge func
//this is used instead of having to each of these variables one by one into the func which is ugly
type processChargeInputs struct {
	context              context.Context
	amountCents          uint64
	invoiceNum           string
	poNum                string
	companyData          company.Info
	customerData         CustomerDatastore
	userProcessingCharge string
	autoChargeReferrer   string
	autoChargeReason     string
	authorizeOnly        bool
	level3Params         chargeLevel3ParamsJSON
	level3Provided       bool
}

//chargeLevel3ParamsJSON is the set of parameters that can be used for the Level III data.
//**Need to copy these to get unmarshalling to work properly since Stripe use `form` struct tags.
type chargeLevel3ParamsJSON struct {
	CustomerReference  string                            `json:"customer_reference"`
	LineItems          []chargeLevel3LineItemsParamsJSON `json:"line_items"`
	MerchantReference  string                            `json:"merchant_reference"`
	ShippingAddressZip string                            `json:"shipping_address_zip"`
	ShippingFromZip    string                            `json:"shipping_from_zip"`
	ShippingAmount     int64                             `json:"shipping_amount"`
}

//chargeLevel3LineItemsParamsJSON is the set of parameters that represent a line item on level III data.
//**Need to copy these to get unmarshalling to work properly since Stripe use `form` struct tags.
type chargeLevel3LineItemsParamsJSON struct {
	DiscountAmount     int64  `json:"discount_amount"`
	ProductCode        string `json:"product_code"`
	ProductDescription string `json:"product_description"`
	Quantity           int64  `json:"quantity"`
	TaxAmount          int64  `json:"tax_amount"`
	UnitCost           int64  `json:"unit_cost"`
}

//ManualCharge processes a charge on a credit card
//this is used when a user clicks the charge button in the gui
func ManualCharge(w http.ResponseWriter, r *http.Request) {
	//get inputs
	datastoreID, _ := strconv.ParseInt(r.FormValue("datastoreId"), 10, 64) //id from datastore
	amount := r.FormValue("amount")                                        //in dollars
	invoice := r.FormValue("invoice")
	poNum := r.FormValue("po")
	chargeAndRemove, _ := strconv.ParseBool(r.FormValue("chargeAndRemove")) //true if card should be removed after charging
	authorizeOnly, _ := strconv.ParseBool(r.FormValue("authorizeOnly"))     //true if we don't want to capture the card, just check if funds are available

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
		output.Error(errMissingStatementDescriptor, "Your company does not have a statement descriptor set.  Please ask an admin to set one.", w)
		return
	}

	inputs := processChargeInputs{
		context:              c,
		amountCents:          amountCents,
		invoiceNum:           invoice,
		poNum:                poNum,
		companyData:          companyInfo,
		customerData:         custData,
		userProcessingCharge: username,
		autoChargeReferrer:   "",
		autoChargeReason:     "",
		authorizeOnly:        authorizeOnly,
	}
	out, errMsg, err := processCharge(inputs)
	if err != nil {
		output.Error(err, errMsg, w)
		return
	}

	//charge successful
	//check if we need to remove this card
	//remove it if necessary
	if chargeAndRemove {
		err := remove(c, datastoreID)
		if err != nil {
			log.Println("Error removing card after charge.", err)
		}
	}

	output.Success("cardCharged", out, w)
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

	//above inputs are the same for manual or auto charges
	//below are for auto charges only
	apiKey := r.FormValue("api_key")
	autoCharge, _ := strconv.ParseBool(r.FormValue("auto_charge"))         //true if we should actually charge the card, false for testing
	referrer := r.FormValue("auto_charge_referrer")                        //the name or other identifier for the app making this request to charge the card
	reason := r.FormValue("auto_charge_reason")                            //the action or other identifier within the app making this request (if the referrer has many actions to charge a card, this lets you figure out which action charged the card)
	level3Params := r.FormValue("level3_params")                           //level3 card processing details in json format, only provided if level3Provided is true
	level3Provided, _ := strconv.ParseBool(r.FormValue("level3_provided")) //level3 card details were provided

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
	if len(reason) == 0 {
		output.Error(errMissingInput, "There was no 'reason' given.  This should be the function of the app that made this auto-charge request.  This is used for logging.", w)
		return
	}
	if len(apiKey) == 0 {
		output.Error(errMissingAPIKey, "There was no api given. This must be given in the 'api_key' field to authenticate this request.", w)
		return
	}

	//verify api key
	settings, err := appsettings.Get(r)
	if err != nil {
		output.Error(err, "Could not get app settings to verify api key.", w)
		return
	}
	if settings.APIKey != apiKey {
		output.Error(errInvalidAPIKey, "The api key provided in the request is not correct.", w)
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
		output.Error(errMissingStatementDescriptor, "Your company does not have a statement descriptor set.  Please ask an admin to set one.", w)
		return
	}

	//build charge request data
	inputs := processChargeInputs{
		context:              c,
		amountCents:          amountCents,
		invoiceNum:           invoice,
		poNum:                poNum,
		companyData:          companyInfo,
		customerData:         custData,
		userProcessingCharge: "api",
		autoChargeReferrer:   referrer,
		autoChargeReason:     reason,
		authorizeOnly:        false,
	}

	//check if level3 data was provided
	//parse level3 data into struct and add it to charge data
	if level3Provided {
		var l3Params chargeLevel3ParamsJSON
		err := json.Unmarshal([]byte(level3Params), &l3Params)
		if err != nil {
			log.Println("could not unmarshal level 3 params, continuing without them", err)
		} else {
			inputs.level3Params = l3Params
			inputs.level3Provided = true
		}
	}

	//process the charge
	out, errMsg, err := processCharge(inputs)
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
func processCharge(input processChargeInputs) (out chargeSuccessful, errMsg string, err error) {
	//get stripe client
	sc := CreateStripeClient(input.context)

	//check if invoice or po number are blank
	//so that the description on stripe's dashboard makes sense if values are missing
	if len(input.invoiceNum) == 0 {
		input.invoiceNum = "*not provided*"
	}
	if len(input.poNum) == 0 {
		input.poNum = "*not provided*"
	}

	//capture is the opposite of authorize
	capture := !input.authorizeOnly

	//build charge object
	chargeParams := &stripe.ChargeParams{
		Customer:            stripe.String(input.customerData.StripeCustomerToken),
		Amount:              stripe.Int64(int64(input.amountCents)),
		Currency:            stripe.String(currency),
		Description:         stripe.String("Charge for invoice: " + input.invoiceNum + ", purchase order: " + input.poNum + "."),
		StatementDescriptor: stripe.String(input.companyData.StatementDescriptor),
		Capture:             stripe.Bool(capture),
	}

	//add metadata
	chargeParams.AddMetadata("customer_name", input.customerData.CustomerName)
	chargeParams.AddMetadata("customer_id", input.customerData.CustomerID)
	chargeParams.AddMetadata("invoice_num", input.invoiceNum)
	chargeParams.AddMetadata("po_num", input.poNum)

	//add level 3 data if needed
	//We have to repackage all the data in a Stripe format since stripe uses *string instead of
	//string and uses `form` struct tags instead of `json` which causes some issues.
	//Have to chop inputs to correct length per https://stripe.com/docs/level3.
	if input.level3Provided {
		chargeParams.AddMetadata("level3_provided", "true")

		if len(input.level3Params.CustomerReference) > 17 {
			input.level3Params.CustomerReference = input.level3Params.CustomerReference[:17]
		}
		if len(input.level3Params.MerchantReference) > 25 {
			input.level3Params.MerchantReference = input.level3Params.MerchantReference[:25]
		}

		l3 := stripe.ChargeLevel3Params{
			CustomerReference:  stripe.String(input.level3Params.CustomerReference),
			MerchantReference:  stripe.String(input.level3Params.MerchantReference),
			ShippingAddressZip: stripe.String(input.level3Params.ShippingAddressZip),
			ShippingFromZip:    stripe.String(input.level3Params.ShippingFromZip),
			ShippingAmount:     stripe.Int64(input.level3Params.ShippingAmount),
		}

		l3Items := []*stripe.ChargeLevel3LineItemsParams{}
		for _, v := range input.level3Params.LineItems {
			if len(v.ProductCode) > 12 {
				v.ProductCode = v.ProductCode[:12]
			}
			if len(v.ProductDescription) > 26 {
				v.ProductDescription = v.ProductDescription[:26]
			}

			item := stripe.ChargeLevel3LineItemsParams{
				DiscountAmount:     stripe.Int64(v.DiscountAmount),
				Quantity:           stripe.Int64(v.Quantity),
				TaxAmount:          stripe.Int64(v.TaxAmount),
				UnitCost:           stripe.Int64(v.UnitCost),
				ProductCode:        stripe.String(v.ProductCode),
				ProductDescription: stripe.String(v.ProductDescription),
			}

			l3Items = append(l3Items, &item)
		}

		l3.LineItems = l3Items
		chargeParams.Level3 = &l3
	}

	if input.authorizeOnly {
		chargeParams.AddMetadata("authorized_by", input.userProcessingCharge)
		chargeParams.AddMetadata("authorized_date", timestamps.ISO8601())
	} else {
		chargeParams.AddMetadata("processed_by", input.userProcessingCharge)
	}

	if input.userProcessingCharge == "api" {
		chargeParams.AddMetadata("auto_charge", "true")
		chargeParams.AddMetadata("auto_charge_referrer", input.autoChargeReferrer)
		chargeParams.AddMetadata("auto_charge_reason", input.autoChargeReason)
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
			//err returned form sc.Charges.New is a struct/json.
			//extract the actual error message for err.  prepend with "stripe:" so we know where this error came from
			//use the textual error message for errMsg but add some text for context in other apps
			stripeErr := err.(*stripe.Error)
			err = errors.New("stripe: " + string(stripeErr.Type))
			errMsg = "Stripe returned an error: " + stripeErr.Msg + " (" + string(stripeErr.Code) + ")"

			log.Println("card.charge: stripe.Error")
			log.Printf("%+v", stripeErr)
			return
		}
	}

	//update the last charge/used timestamp
	//don't return on an error since this isn't a huge issue if it doesn't work
	err = updateCardLastUsed(input.context, input.customerData.ID)
	if err != nil {
		log.Println("card.processCharge - could not update card last used", err)
		err = nil
	}

	//build struct to output a success message to the client
	out = chargeSuccessful{
		CustomerName:   input.customerData.CustomerName,
		Cardholder:     input.customerData.Cardholder,
		CardExpiration: input.customerData.CardExpiration,
		CardLast4:      input.customerData.CardLast4,
		Amount:         strconv.Itoa(int(input.amountCents) / 100),
		Invoice:        input.invoiceNum,
		Po:             input.poNum,
		Datetime:       timestamps.ISO8601(),
		ChargeID:       chg.ID,
		AuthorizedOnly: input.authorizeOnly,
	}
	return
}

//Capture captures a previous authorized charge
func Capture(w http.ResponseWriter, r *http.Request) {
	//get input
	chargeID := strings.TrimSpace(r.FormValue("chargeID"))

	//get stripe client
	ctx := r.Context()
	sc := CreateStripeClient(ctx)

	//get username of logged in user
	//we record this data so we can see who processed a charge in the reports
	username := sessionutils.GetUsername(r)

	//update the charge with some notes
	params := &stripe.ChargeParams{}
	params.AddMetadata("processed_by", username)
	params.AddMetadata("processed_date", timestamps.ISO8601())
	_, err := sc.Charges.Update(chargeID, params)
	if err != nil {
		log.Println("card.Capture: error updating charge", err)
		//we don't return here since if we can't update the charge it isn't the worse thing in the world
	}

	//capture the charge
	//this actual charges the card
	_, err = sc.Charges.Capture(chargeID, nil)
	if err != nil {
		output.Error(err, "Could not capture charge.", w)
		return
	}

	output.Success("cardCharged", nil, w)
	return
}

//updateCardLastUsed updates the LastUsedTimestamp of a card
func updateCardLastUsed(ctx context.Context, customerID int64) error {
	//use correct db
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		q := `
			UPDATE ` + sqliteutils.TableCards + `
			SET LastUsedTimestamp=?
			WHERE ID=?
		`
		stmt, err := c.Prepare(q)
		if err != nil {
			return err
		}

		_, err = stmt.Exec(
			timestamps.Unix(),
			customerID,
		)

	} else {
		fullKey := datastoreutils.GetKeyFromID(datastoreutils.EntityCards, customerID)
		client, err := datastoreutils.Connect(ctx)
		if err != nil {
			return err
		}

		//look up card info first since datastore can't do updates
		cardData := CustomerDatastore{}
		err = client.Get(ctx, fullKey, &cardData)
		if err != nil {
			return err
		}

		//update timestamp
		cardData.LastUsedTimestamp = timestamps.Unix()
		_, err = client.Put(ctx, fullKey, &cardData)
	}

	return nil
}
