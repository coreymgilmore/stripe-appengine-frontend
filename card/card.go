package card 

import (
	"net/http"
	"io/ioutil"
	"errors"
	"strconv"
	"time"
	"math"

	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/urlfetch"
	
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/refund"
	"github.com/coreymgilmore/timestamps"
	
	"output"
	"memcacheutils"
	"sessionutils"
	"chargeutils"
	"templates"
	"users"
)

const (
	//PATH TO PRIVATE KEY AND STATEMENT DESCRIPTOR
	//stored in separate text files so they are easily changed without having to edit code
	//values are read into the app upon initializing
	STRIPE_PRIVATE_KEY_PATH = 		"config/stripe-secret-key.txt"
	STRIPE_STATEMENT_DESC_PATH = 	"config/statement-descriptor.txt"

	DATASTORE_KIND = 				"card"
	LIST_OF_CARDS_KEYNAME = 		"list-of-cards"
	CURRENCY = 						"usd"
	MIN_CHARGE = 					50	//cents
)

var (
	stripePrivateKey = 			""
	stripeStatementDescriptor = ""
	initError error

	ErrStripeKeyTooShort = 		errors.New("The Stripe private key ('stripe-secret-key.txt') file was empty. Please provide your Stripe secret key.")
	ErrStatementDescMissing = 	errors.New("The statement descriptor ('statement-descriptor.txt') file was empty. Please provide a statement descriptor.")
	ErrMissingCustomerName = 	errors.New("missingCustomerName")
	ErrMissingCardholerName = 	errors.New("missingCardholderName")
	ErrMissingCardToken = 		errors.New("missingCardToken")
	ErrMissingExpiration = 		errors.New("missingExpiration")
	ErrMissingLast4 = 			errors.New("missingLast4CardDigits")
	ErrStripe =					errors.New("stripeError")
	ErrMissingInput = 			errors.New("missingInput")
	ErrChargeAmountTooLow = 	errors.New("amountLessThanMinCharge")
	ErrCustomerNotFound = 		errors.New("customerNotFound")
	ErrCustIdAlreadyExists = 	errors.New("customerIdAlreadyExists")
)

//SAVING CUSTOMER TO DATASTORE
type CustomerDatastore struct {
	CustomerId 			string 	`json:"customer_id"`
	CustomerName 		string 	`json:"customer_name"`
	Cardholder 			string 	`json:"cardholder_name"`
	CardExpiration 		string 	`json:"card_expiration"`
	CardLast4 			string 	`json:"card_last4"`
	StripeCustomerToken string 	`json:"-"`
	DatetimeCreated 	string 	`json:"-"`
}

//CONFIRMING CUSTOMER WAS SAVED
type confirmCustomer struct{
	CustomerName 		string
	Cardholder 			string
	CardExpiration 		string
	CardLast4 			string	
}

//CHARGE SUCCESSFUL, RETURN DATA TO CLIENT
type chargeSuccessful struct{
	CustomerName 		string 	`json:"customer_name"`
	Cardholder 			string 	`json:"cardholder_name"`
	CardExpiration 		string 	`json:"card_expiration"`
	CardLast4 			string 	`json:"card_last4"`
	Amount 				string 	`json:"amount"`
	Invoice 			string 	`json:"invoice"`
	Po 					string 	`json:"po"`
	Datetime 			string 	`json:"datetime"`
	ChargeId 			string 	`json:"charge_id"`
}

//FOR RETURNING JUST A LIST OF CARDS
//used to build autocomplete datalist in html
type CardList struct {
	CustomerName 		string	`json:"customer_name"`
	Id 					int64 	`json:"id"`
}

//FOR BUILDING REPORTS
type reportData struct{
	UserData 		users.User 					`json:"user_data"`
	StartDate 		time.Time					`json:"start_datetime"`
	EndDate 		time.Time					`json:"end_datetime"`
	Charges 		[]chargeutils.Data 			`json:"charges"`
	Refunds 		[]chargeutils.RefundData 	`json:"refunds"`
	TotalAmount 	string 						`json:"total_amount"`
	NumCharges  	uint16						`json:"num_charges"`
}

//**********************************************************************
//INIT
//read stripe private key and statement descriptor from config files
//save values to variable for use in other functions
func Init() error {
	//stripe private key
	apikey, err := ioutil.ReadFile(STRIPE_PRIVATE_KEY_PATH)
	if err != nil {
		initError = err
		return err
	}

	//save key to session
	//save key to stripe package for use
	stripePrivateKey = 	string(apikey)
	stripe.Key = 		stripePrivateKey

	//statement descriptor
	descriptor, err := ioutil.ReadFile(STRIPE_STATEMENT_DESC_PATH)
	if err != nil {
		initError = err
		return err
	}

	//save descripto to variable for use when charging
	stripeStatementDescriptor = string(descriptor)

	return nil
}

//**********************************************************************
//HANDLE HTTP REQUESTS

//GET LIST OF CARDS
//returns list of card ids (datastore id) and customer names
func GetAll(w http.ResponseWriter, r *http.Request) {
	//memcache
	result := 	make([]CardList, 0, 25)
	c := 		appengine.NewContext(r)
	_, err := 	memcache.Gob.Get(c, LIST_OF_CARDS_KEYNAME, &result)
	if err == nil {
		output.Success("Cardlist-cached", result, w)
		return
	}

	//look up list of cards from datastore
	if err == memcache.ErrCacheMiss {
		q := 			datastore.NewQuery(DATASTORE_KIND).Order("CustomerName").Project("CustomerName")
		cards := 		make([]CustomerDatastore, 0, 25)
		keys, err := 	q.GetAll(c, &cards)
		if err != nil {
			output.Error(err, "Error retrieving list of cards from datastore.", w)
			return
		}

		//build result
		idAndNames := make([]CardList, 0, 25)
		for i, r := range cards {
			x := CardList{
				CustomerName: 	r.CustomerName,
				Id: 			keys[i].IntID(),
			}

			idAndNames = append(idAndNames, x)
		}

		//save list of cards to memcache
		//ignore errors since we already got results
		memcacheutils.Save(c, LIST_OF_CARDS_KEYNAME, idAndNames)

		//return data to client
		output.Success("CardList", idAndNames, w)
		return
	
	} else if err != nil {
		output.Error(err, "Unknown error retrieving list of cards.", w)
		return
	}

	return
}

//GET INFO ON ONE CARD
//returns all the data for a given card id (datastore id)
func GetOne(w http.ResponseWriter, r *http.Request) {
	//get form value
	datastoreId := 		r.FormValue("customerId")
	datstoreIdInt, _ := strconv.ParseInt(datastoreId, 10, 64)

	//get customer card data
	c := 			appengine.NewContext(r)
	data, err := 	findByDatastoreId(c, datstoreIdInt)
	if err != nil {
		output.Error(err, "Could not find this customer's data.", w)
		return
	}

	output.Success("cardFound", data, w)
	return
}

//ADD A NEW CARD TO THE DATASTORE
//stripe created a card token (with stripe.js) that can only be used once
//need to create a stripe customer to charge many times
//create the customer and save the stripe customer token along with the customer id and customer name to datastore
//the customer name is used to look up the stripe customer token that is used to charge the card
func Add(w http.ResponseWriter, r *http.Request) {
	//get form values
	customerId := 	r.FormValue("customerId")
	customerName := r.FormValue("customerName")
	cardholder := 	r.FormValue("cardholder")
	cardToken := 	r.FormValue("cardToken")
	cardExp :=		r.FormValue("cardExp")
	cardLast4 := 	r.FormValue("cardLast4")

	//make sure all form values were given
	if len(customerName) == 0 {
		output.Error(ErrMissingCustomerName, "You did not provide the customer's name.", w)
		return
	}
	if len(cardholder) == 0 {
		output.Error(ErrMissingCustomerName, "You did not provide the cardholer's name.", w)
		return
	}
	if len(cardToken) == 0 {
		output.Error(ErrMissingCardToken, "A serious error occured, the card token is missing. Please contact an administrator.", w)
		return
	}

	//these are the returned values from stripe.js and are just used to identify a card
	//the only info we need to charge a card is the card token
	//these just show the user who is charging the card some info to verify they are charging the correct card
	if len(cardExp) == 0 {
		output.Error(ErrMissingExpiration, "The card's expiration date is missing.", w)
		return
	}
	if len(cardLast4) == 0 {
		output.Error(ErrMissingLast4, "The card's last four digits are missing.", w)
		return
	}

	//if customer id was given, make sure a customer with this ID does not already exist
	//customer id is unique and is the basis for api-like calls to /main/ that autofills the charge card panel
	c := appengine.NewContext(r)
	if len(customerId) != 0 {
		_, err := 	FindByCustId(c, customerId)
		if err == nil {
			//customer already exists
			output.Error(ErrCustIdAlreadyExists, "This customer ID is already in use. Please double check your records or remove the customer with this customer ID first.", w)
			return
		} else if err != ErrCustomerNotFound {
			output.Error(err, "An error occured while verifying this customer ID does not already exist. Please try again or leave the customer ID blank.", w)
			return
		}
	}

	//init stripe
	sc := createAppengineStripeClient(c)

	//create the stripe customer
	custParams := &stripe.CustomerParams{Desc: customerName}
	custParams.SetSource(cardToken)
	cust, err := sc.Customers.New(custParams)
	if err != nil {
		stripeErr := 		err.(*stripe.Error)
		stripeErrMsg := 	stripeErr.Msg
		output.Error(ErrStripe, stripeErrMsg, w)
		return
	}

	//customer created on stripe
	//save to datastore
	newCustomer := CustomerDatastore{
		CustomerId: 			customerId,
		CustomerName: 			customerName,
		Cardholder: 			cardholder,
		CardExpiration: 		cardExp,
		CardLast4: 				cardLast4,
		StripeCustomerToken: 	cust.ID,
		DatetimeCreated: 		timestamps.ISO8601(),
	}

	incompleteKey := 	createNewCustomerKey(c)
	_, err = 			save(c, incompleteKey, newCustomer)
	if err != nil {
		output.Error(err, "There was an error while saving this customer. Please try again.", w)
		return
	}

	//customer saved
	//return okay
	confirmation := confirmCustomer{
		CustomerName: 			customerName,
		Cardholder: 			cardholder,
		CardExpiration: 		cardExp,
		CardLast4: 				cardLast4,
	}
	output.Success("createCustomer", confirmation, w)

	//clear list of cards from memcache
	//since a card is added, clients need to rebuild list of cards
	memcacheutils.Delete(c, LIST_OF_CARDS_KEYNAME)
	return
}

//REMOVE A CARD
//remove from memcache, datastore, and stripe
func Remove(w http.ResponseWriter, r *http.Request) {
	//get form values
	custId := 		r.FormValue("customerId")
	custIdInt, _ := strconv.ParseInt(custId, 10, 64)

	//validation
	if len(custId) == 0 {
		output.Error(ErrMissingInput, "A customer's datastore ID must be given but was missing. This value is different from your \"Customer ID\" and should have been submitted automatically.", w)
		return
	}

	//look up customer's stripe id
	c := 				appengine.NewContext(r)
	custData, err := 	findByDatastoreId(c, custIdInt)
	if err != nil {
		output.Error(err, "An error occured while trying to look up customer's Stripe information.", w)
	}

	//init stripe
	sc := createAppengineStripeClient(c)

	//remove card from stripe
	//ingnore errors with .Del() b/c as long as we delete the customer from the datastore any users should not be able to charge this customer card
	stripeId := custData.StripeCustomerToken
	sc.Customers.Del(stripeId)

	//remove card from memcache
	//delete list of cards in memcache
	memcache.Delete(c, custId)
	memcache.Delete(c, LIST_OF_CARDS_KEYNAME)

	//remove from datastore
	completeKey := getCustomerKeyFromId(c, custIdInt)
	err = datastore.Delete(c, completeKey)
	if err != nil {
		output.Error(err, "There was an error while trying to delete this customer. Please try again.", w)
		return
	}

	//customer remove
	output.Success("removeCustomer", nil, w)
	return
}

//CHARGE A CARD
func Charge(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreId := 		r.FormValue("datastoreId")
	customerName := 	r.FormValue("customerName")
	amount := 			r.FormValue("amount")
	invoice := 			r.FormValue("invoice")
	poNum := 			r.FormValue("po")

	//validation
	if len(datastoreId) == 0 {
		output.Error(ErrMissingInput, "A customer ID should have been submitted automatically but was not. Please contact an administrator.", w)
		return
	}
	if len(amount) == 0 {
		output.Error(ErrMissingInput, "No amount was provided. You cannot charge a card nothing!", w)
		return
	}

	//get amount as a integer in cents
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w)
		return
	}
	amountCents := uint64(amountFloat * 100)

	//check if amount is greater than zero
	if amountCents < MIN_CHARGE {
		output.Error(ErrChargeAmountTooLow, "You must charge at least " + strconv.FormatInt(MIN_CHARGE, 10) + " cents.", w)
		return
	}

	//look up stripe customer id from datastore
	c := 					appengine.NewContext(r)
	datastoreIdInt, _ := 	strconv.ParseInt(datastoreId, 10, 64)
	custData, err := 		findByDatastoreId(c, datastoreIdInt)
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
	session := 	sessionutils.Get(r)
	username := session.Values["username"].(string)

	//init stripe
	sc := createAppengineStripeClient(c)

	//init charge
	chargeParams := &stripe.ChargeParams{
		Customer: 	custData.StripeCustomerToken,
		Amount: 	amountCents,
		Currency: 	CURRENCY,
		Desc: 		"Charge for invoice: " + invoice + ", purchase order: " + poNum + ".",
		Statement: 	formatStatementDescriptor(),
	}

	//add metadata to charge
	chargeParams.AddMeta("customer_name", customerName)
	chargeParams.AddMeta("datastore_id", datastoreId)
	chargeParams.AddMeta("customer_id", custData.CustomerId)
	chargeParams.AddMeta("invoice_num", invoice)
	chargeParams.AddMeta("po_num", poNum)
	chargeParams.AddMeta("charged_by", username)

	//create the charge
	chg, err := sc.Charges.New(chargeParams)
	if err != nil {
		stripeErr := 		err.(*stripe.Error)
		stripeErrMsg := 	stripeErr.Msg
		output.Error(ErrStripe, stripeErrMsg, w)
		return
	}

	//charge completed successfully
	out := chargeSuccessful{
		CustomerName: 		customerName,
		Cardholder: 		custData.Cardholder,
		CardExpiration: 	custData.CardExpiration,
		CardLast4: 			custData.CardLast4,
		Amount: 			amount,
		Invoice: 			invoice,
		Po: 				poNum,
		Datetime: 			timestamps.ISO8601(),
		ChargeId: 			chg.ID,
	}

	//save charge to memcache
	//less data to get from stripe if receipt is needed
	memcacheutils.Save(c, chg.ID, chg)

	//return to ajax call
	//show success panel
	output.Success("cardCharged", out, w)
	return
}

//SHOW REPORTS
//results array of full stripe Charge objects
func Report(w http.ResponseWriter, r *http.Request) {
	//get form valuess
	datastoreId := 	r.FormValue("customer-id")
	startString := 	r.FormValue("start-date")
	endString := 	r.FormValue("end-date")
	hoursToUTC := 	r.FormValue("timezone") 	//hour offset from UTC (EST is -4)

	//get report data form stripe
	//make sure inputs are given
	if len(startString) == 0 {
		output.Error(ErrMissingInput, "You must supply a 'start-date'.", w)
		return
	}
	if len(endString) == 0 {
		output.Error(ErrMissingInput, "You must supply a 'end-date'.", w)
		return
	}
	if len(hoursToUTC) == 0 {
		output.Error(ErrMissingInput, "You must supply a 'timezone'.", w)
		return
	}

	//get timezone offset
	//adjust for the local timezone the user is in
	//hoursToUTC is a number generated by JS (-4 for EST)
	tzOffset := calcTzOffset(hoursToUTC)

	//get datetimes from provided strings
	startDt, err := time.Parse("2006-01-02 -0700", startString + " " + tzOffset)
	if err != nil {
		output.Error(err, "Could not convert start date to a timestamp.", w)
		return
	}
	endDt, err := time.Parse("2006-01-02 -0700", endString + " " + tzOffset)
	if err != nil {
		output.Error(err, "Could not convert end date to a timestamp.", w)
		return
	}

	//get end of day datetime
	//need to get 23:59:59
	endDt = endDt.Add((24 * 60 - 1) * time.Minute + (59 * time.Second))

	//get unix timestamps
	startUnix := 	startDt.Unix()
	endUnix := 		endDt.Unix()
	
	//init stripe
	c := 	appengine.NewContext(r)
	sc := 	createAppengineStripeClient(c)	

	//retrieve data from stripe
	//date is a range inclusive of the days the user chose
	//limit is 200 but apparently 100 is the max per stripe
	params := &stripe.ChargeListParams{}
	params.Filters.AddFilter("created", "gte", strconv.FormatInt(startUnix, 10))
	params.Filters.AddFilter("created", "lte", strconv.FormatInt(endUnix, 10))
	params.Filters.AddFilter("limit", "", "100")

	//check if we need to filter by a specific customer/card
	//look up stripe customer id by the datastore id
	if len(datastoreId) != 0 {
		datastoreIdInt, _ := 	strconv.ParseInt(datastoreId, 10, 64)
		custData, err := 		findByDatastoreId(c, datastoreIdInt)
		if err != nil {
			output.Error(err, "An error occured and this report could not be generated.", w)
			return
		}

		params.Filters.AddFilter("customer", "", custData.StripeCustomerToken)
	}

	//get results
	//loop through each charge and extract charge data
	//add up total amount of all charges
	charges := 					sc.Charges.List(params)
	data := 					make([]chargeutils.Data, 0, 10)
	var amountTotal uint64 = 	0
	var numCharges uint16 = 	0
	for charges.Next() {
		//get each charges data
		chg := 	charges.Charge()
		d := 	chargeutils.ExtractData(chg)
		data = 	append(data, d)

		//increment totals
		amountTotal += d.AmountCents
		numCharges++
	}

	//convert total amount to dollars
	amountTotalDollars := strconv.FormatFloat((float64(amountTotal) / 100), 'f', 2, 64)

	//retrieve refunds
	//have to type assert everything b/c Stripe just returns a json aka map[string]interface{}...pain in the ass
	eventParams := &stripe.EventListParams{}
	eventParams.Filters.AddFilter("created", "gte", strconv.FormatInt(startUnix, 10))
	eventParams.Filters.AddFilter("created", "lte", strconv.FormatInt(endUnix, 10))
	eventParams.Filters.AddFilter("limit", "", "100")
	eventParams.Filters.AddFilter("type", "", "charge.refunded")
	
	events := 	sc.Events.List(eventParams)
	refunds := 	chargeutils.ExtractRefunds(events)

	//get logged in user's data
	//for determining if receipt/refund buttons need to be hidden
	session := 		sessionutils.Get(r)
	userId := 		session.Values["user_id"].(int64)
	userdata, _ := 	users.Find(c, userId)

	//store data for building template
	result := reportData{
		UserData: 		userdata,
		StartDate: 		startDt,
		EndDate: 		endDt,
		Charges: 		data,
		Refunds: 		refunds,
		TotalAmount: 	amountTotalDollars,
		NumCharges: 	numCharges,
	}

	//build template to display report
	templates.Load(w, "report", result)
	return
}

//REFUND A CHARGE
func Refund(w http.ResponseWriter, r *http.Request) {
	//get form values
	chargeId := 	r.FormValue("chargeId")
	amount := 		r.FormValue("amount")
	reason := 		r.FormValue("reason")

	//make sure inputs were given
	if len(chargeId) == 0 {
		output.Error(ErrMissingInput, "A charge ID was not provided. This is a serious error. Please contact an administrator.", w)
		return
	}
	if len(amount) == 0 {
		output.Error(ErrMissingInput, "No amount was given to refund.", w)
		return
	}

	//convert amount to cents
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		output.Error(err, "The amount you provided could not be converted to a refund.", w)
		return
	}
	amountCents := uint64(amountFloat * 100)

	//build refund
	params := &stripe.RefundParams{
		Charge: chargeId,
		Amount: amountCents,
	}

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
		stripeErr := 		err.(*stripe.Error)
		stripeErrMsg := 	stripeErr.Msg
		output.Error(ErrStripe, stripeErrMsg, w)
		return
	}

	//done
	output.Success("refund-done", nil, w)
	return
}

//**********************************************************************
//DATASTORE KEYS

//CREATE INCOMPLETE KEY
func createNewCustomerKey(c appengine.Context) *datastore.Key {
	return datastore.NewIncompleteKey(c, DATASTORE_KIND, nil)
}

//CREATE COMPLETE KEY FOR USER
//get the full complete key from just the ID of a key
func getCustomerKeyFromId(c appengine.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, DATASTORE_KIND, "", id, nil)
}

//**********************************************************************
//FUNCS

//CHECK IF STRIPE KEY WAS READ CORRECTLY
func CheckStripe() error {
	//check if reading key from file returned an error
	if initError != nil {
		return initError
	}

	//check if there was actually some text read
	if len(stripePrivateKey) == 0 {
		return ErrStripeKeyTooShort
	}

	//private key read correctly
	return nil
}

//SAVE CARD
func save(c appengine.Context, key *datastore.Key, customer CustomerDatastore) (*datastore.Key, error) {
	//save customer
	completeKey, err := datastore.Put(c, key, &customer)
	if err != nil {
		return key, err
	}

	//save customer to memcache
	memcacheKey := strconv.FormatInt(completeKey.IntID(), 10)
	err = memcacheutils.Save(c, memcacheKey, customer)
	if err != nil {
		return completeKey, err
	}

	//done
	return completeKey, nil
}

//GET CARD DATA BY THE DATASTORE ID
//use the datastore intID as the id when displayed in htmls
func findByDatastoreId(c appengine.Context, datastoreId int64) (CustomerDatastore, error) {
	//memcache
	//convert datastoreId to string for memcache
	var r CustomerDatastore
	datastoreIdStr := 	strconv.FormatInt(datastoreId, 10)
	_, err := 			memcache.Gob.Get(c, datastoreIdStr, &r)
	
	if err == nil {
		return r, nil
	} else if err == memcache.ErrCacheMiss {
		//look up data in datastore
		key := 			getCustomerKeyFromId(c, datastoreId)
		data, err := 	datastoreFindOne(c, "__key__ =", key, 1, []string{"CustomerName", "Cardholder", "CardLast4", "CardExpiration", "StripeCustomerToken"})
		if err != nil {
			return data, err
		}

		//save to memcache
		memcacheutils.Save(c, datastoreIdStr, data)

		//done
		return data, nil
	} else {
		return CustomerDatastore{}, err
	}
}

//FIND CARD DATA BY CUSTOMER ID
//customer id is the value provided during "add a new card" and is unique to the company processing credit cards
//this is used when making an api-like request to load the /main/ page with the card's data automatically
func FindByCustId(c appengine.Context, customerId string) (CustomerDatastore, error) {
	//memcache
	var r CustomerDatastore
	_, err := memcache.Gob.Get(c, customerId, &r)
	
	if err == nil {
		return r, nil
	} else if err == memcache.ErrCacheMiss {
		//look up data in datastore
		data, err := datastoreFindOne(c, "CustomerId =", customerId, 1, []string{"CustomerName", "Cardholder", "CardLast4", "CardExpiration"})
		if err != nil {
			return data, err
		}

		//save to memcache
		memcacheutils.Save(c, customerId, data)

		//done
		return data, nil
	} else {
		return CustomerDatastore{}, err
	}
}

//FIND AN ENTITY IN THE DATASTORE
func datastoreFindOne(c appengine.Context, filterField string, filterValue interface{}, limit int, project []string) (CustomerDatastore, error) {
	q := datastore.NewQuery(DATASTORE_KIND).Filter(filterField, filterValue).Limit(limit).Project(project...)
	r := make([]CustomerDatastore, 0, 1)

	_, err := q.GetAll(c, &r)
	if err != nil {
		return CustomerDatastore{}, err
	}

	//check if one result exists
	if len(r) == 0 {
		return CustomerDatastore{}, ErrCustomerNotFound
	}

	//get one result
	return r[0], nil
}

//FORMAT STATEMENT DESCRIPTOR
//this is a max of 22 characters long
func formatStatementDescriptor() string {
	s := stripeStatementDescriptor

	if len(s) > 22 {
		return s[:22]
	}

	return s
}

//GET A GOLANG STYLE TIMEZONE OFFSET
//takes a string value input of the hours before or after UTC (-4 for EST for example)
//the input generated via JS [var d = new Date(); (d.getTimezoneOffset() / 60 * -1);]
//outputs a string in the format: -0400
func calcTzOffset(hoursToUTC string) string {
	//placeholder for output
	var tzOffset = ""

	//get hours as a float
	hoursFloat, _ := strconv.ParseFloat(hoursToUTC, 64)

	//check if hours is before or after UTC
	//negative numbers are behind UTC (aka EST is -4 hours behind UTC)
	if hoursFloat > 0 {
		tzOffset += "+"
	} else {
		tzOffset += "-"
	}

	//need to pad with zeros in front if the number is only one digit
	//need absolute value of input hoursToUTC b/c + or - symbol is already added
	absHours := math.Abs(hoursFloat)
	if absHours < 10 {
		tzOffset += "0"
	}

	//add hours to offset
	//must use abs value here since it will not have + or - symbol
	tzOffset += strconv.FormatFloat(absHours, 'f', 0, 64)

	//pad with zeros behind
	//final format must be four digits (0000)
	tzOffset += "00"

	//make sure output is only 5 characters long
	if len(tzOffset) > 5 {
		return tzOffset[:5]
	}

	return tzOffset
}

//CREATE STRIPE CLIENT
//this creates an httpclient on a per-request basis and is used only for this one request
//need to do this since each request needs its own http client backend
//otherwise multiple requests could use the incorrect http client
//this is for app engine only since the golang http.DefaultClient is unavailable
func createAppengineStripeClient(c appengine.Context) *client.API {
	//create http client
	httpClient := urlfetch.Client(c)

	//create backends for http client
	//these are the stripe api backends that we need to communicate with
	backends := stripe.Backends{
		API: 		stripe.BackendConfiguration{stripe.APIBackend, "https://api.stripe.com/v1", httpClient},
		Uploads: 	stripe.BackendConfiguration{stripe.UploadsBackend, "https://uploads.stripe.com/v1", httpClient},
	}

	//initialize the client
	//this creates a client that uses our per-request httpclient
	sc := &client.API{}
	sc.Init(stripePrivateKey, &backends)

	//return the client to use in making charges, adding customer, getting data, etc.
	return sc
}

