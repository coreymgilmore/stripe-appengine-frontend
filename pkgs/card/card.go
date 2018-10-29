/*
Package card implements functionality to add, remove, charge, and issue refunds for credit cards
as well as view reports for lists of charges and refunds.

Charges are performed via Stripe. This package requires loading the Stripe private key for your
Stripe account when the app starts.  The private key should be in the app.yaml file as an
environmental variable.

When a card is added to the app, very minimal (expiration and last 4) of the card's data
is actually stored in App Engine. The card's information is sent to Stripe who then
returns an id for this card. When a charge is processed, this id is sent to Stripe and
Stripe looks up the card's information to charge it. The information stored in App Engine
is safe, as in if someone were to get the data, it could not be used to process charges.
No credit card number is stored.  The data is just used to identify the card so users of
the app know which card they are charging.

Datastore ID: the ID of the entity (think sql "row") in the App Engine Datastore.
Customer ID: the ID that the user provides that links the card to a company.  This is usually from a CRM software.
Stripe ID: the ID stripe uses to process a charge.  Also known as the stripe customer token.
For each datastore ID, there should be one and only one customer ID and stripe ID.
For each customer ID, there can be many datastore IDs and stripe IDs; one for each card added.
For each stripe ID, there should be one and only one datastore ID and customer ID.
*/
package card

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/event"
)

//config is the set of configuraton option for processing charges on cards
//this struct is used when SetConfig is run in package main init()
type config struct {
	StripeSecretKey      string //a 32 character long string starting with "sk_live_" or "sk_test_"
	StripePublishableKey string //a 32 character long string starting with "pk_live_" or "pk_test_"
}

//Config is a copy of the config struct with some defaults set
var Config = config{
	StripeSecretKey:      "",
	StripePublishableKey: "",
}

//stripeKeyLength is the required size of the stripe keys
const stripeKeyLength = 32

const (
	//currency for transactions
	//this should be a simple change for other currencies, but you will need to change the "$" symbol elsewhere in this code base
	currency = "usd"

	//minCharge is the lowest charge the app will allow
	//Stripe takes $0.30 + 2.9% of transactions so it is not worth collecting a charge that will cost us more then we will make
	//this is in cents
	minCharge = 50
)

//configuration errors
var (
	errSecretKeyInvalid      = errors.New("card: the stripe secret key in app.yaml is invalid")
	errPublishableKeyInvalid = errors.New("card: the stripe publishable key in app.yaml is invalid")
)

//other errors
var (
	errMissingCustomerName  = errors.New("card: missing customer name")
	errMissingCardholerName = errors.New("card: missing cardholder name")
	errMissingCardToken     = errors.New("card: missing card token")
	errMissingExpiration    = errors.New("card: missing rxpiration")
	errMissingLast4         = errors.New("card: missing last4 card digits")
	errStripe               = errors.New("card: stripe error")
	errMissingInput         = errors.New("card: missing input")
	errChargeAmountTooLow   = errors.New("card: amount less than min charge")
	errCustomerNotFound     = errors.New("card: customer not found")
	errCustIDAlreadyExists  = errors.New("card: customer id already exists")
)

//SetConfig saves the configuration options for charging cards
//these config options are references in other code directly from the variable Config
func SetConfig(c config) error {
	//validate config options
	secretKey := strings.TrimSpace(c.StripeSecretKey)
	if len(secretKey) != stripeKeyLength {
		return errSecretKeyInvalid
	}

	publishableKey := strings.TrimSpace(c.StripePublishableKey)
	if len(publishableKey) != stripeKeyLength {
		return errSecretKeyInvalid
	}

	//save config to package variable
	Config = c

	return nil
}

//GetAll retrieves the list of all cards in the datastore
//This only gets the datastore id and customer name.
//The data is pulled from the datastore and is returned as json to build
//the datalist drop down where the user can choose what customer to charge.
func GetAll(w http.ResponseWriter, r *http.Request) {
	//connect to datastore
	c := r.Context()
	client, err := datastoreutils.Connect(c)
	if err != nil {
		output.Error(err, "Could not connect to datastore", w)
		return
	}

	//get list from datastore
	//only need to get entity keys and customer names which cuts down on datastore usage
	q := datastore.NewQuery(datastoreutils.EntityCards).Order("CustomerName").Project("CustomerName")
	var cards []CustomerDatastore
	keys, err := client.GetAll(c, q, &cards)
	if err != nil {
		output.Error(err, "Error retrieving list of cards from datastore.", w)
		return
	}

	//build result
	//format data to show just datastore id and customer name
	//creates a map of structs
	var idAndNames []List
	for i, r := range cards {
		x := List{r.CustomerName, keys[i].ID}
		idAndNames = append(idAndNames, x)
	}

	//return data to client
	output.Success("cardList-datastore", idAndNames, w)
	return
}

//GetOne retrieves the full data for one card from the datastore
//This is used to fill in the "charge card" panel with identifying info on
//the card so the user can verify they are charging the correct card.
func GetOne(w http.ResponseWriter, r *http.Request) {
	//get input
	datastoreID, _ := strconv.ParseInt(r.FormValue("customerId"), 10, 64)

	//get customer card data
	c := r.Context()
	data, err := findByDatastoreID(c, datastoreID)
	if err != nil {
		output.Error(err, "Could not find this customer's data.", w)
		return
	}

	//return data to client
	output.Success("cardFound", data, w)
	return
}

//findByDatastoreID retrieves a card's information by its datastore id
//This returns all the info on a card that is needed to build the ui.
func findByDatastoreID(c context.Context, datastoreID int64) (data CustomerDatastore, err error) {
	//connect to datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return
	}

	//get complete key
	key := datastoreutils.GetKeyFromID(datastoreutils.EntityCards, datastoreID)

	//query
	err = client.Get(c, key, &data)
	if err != nil {
		return
	}

	return
}

//FindByCustomerID retrieves a card's information by the unique id from a CRM system
//This id was provided when a card was added to this app.
//This func is used when making api style request to semi-automate the charging of a card.
//only getting the fields we need to show data in the charge card panel
func FindByCustomerID(c context.Context, customerID string) (data CustomerDatastore, err error) {
	//connect to datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return
	}

	//query
	fields := []string{"CustomerName", "Cardholder", "CardLast4", "CardExpiration", "StripeCustomerToken"}
	q := datastore.NewQuery(datastoreutils.EntityCards).Filter("CustomerId =", customerID).Limit(1).Project(fields...)
	i := client.Run(c, q)
	_, err = i.Next(&data)
	if err != nil {
		log.Println("card.FindByCustomerID-1", err)
		return
	}

	return
}

//calcTzOffset takes a string value input of the hours from UTC and outputs a timezone offset usable in golang
//Input is a number such as -4 for EST generated via JS: var d = new Date(); (d.getTimezoneOffset() / 60 * -1);.
//The output is a string in the format "-0400" and is used to construct a golang time.Time.
func calcTzOffset(hoursToUTC string) string {
	//get hours as a float
	hoursFloat, _ := strconv.ParseFloat(hoursToUTC, 64)

	//check if hours is before or after UTC
	//negative numbers are behind UTC (aka EST is -4 hours behind UTC)
	//add leading symbol to tzOffset output
	tzOffset := ""
	if hoursFloat > 0 {
		tzOffset += "+"
	} else {
		tzOffset += "-"
	}

	//need to pad with zeros in front of the number if the number is only one digit long (-9 through 9)
	//add to tzOffset output
	absHours := math.Abs(hoursFloat)
	if absHours < 10 {
		tzOffset += "0"
	}

	//add hours to tzOffset output
	tzOffset += strconv.FormatFloat(absHours, 'f', 0, 64)

	//add trailing zeros to make offset follow format
	tzOffset += "00"

	//make sure output is only 5 characters long
	//(+ or - symbol) + (leading zero if needed) + (hours) + (trailing two zeros)
	if len(tzOffset) > 5 {
		return tzOffset[:5]
	}

	//done
	return tzOffset
}

//createAppendingeStripeClient creates an httpclient on a per-request basis for use in making api calls to Stripe
//Stripe's API is accessed via http requests, need a way to make these requests.
//Urlfetch is the appengine way of making http requests.
//This func returns an httpclient on a per request basis.  Per http request made to this app.
//Otherwise one request could use another requests httpclient which would not be good!
//This is for app engine only since the golang http.DefaultClient is unavailable.
func createAppengineStripeClient(c context.Context) *client.API {
	//create http client
	//returns stripe client to use to process charges
	httpClient := &http.Client{}
	return client.New(Config.StripeSecretKey, stripe.NewBackends(httpClient))
}

//getAmountAsIntCents converts a dollar amount as a string into a cents integer value
//Amounts typed in UI form are dollars as a string (12.56).
//Need amounts as cents to process payments via Stripe (1256).
//Make sure value doesn't add or lose decimal places during type conversions due to
//conversions from floats to int and different precisions.
func getAmountAsIntCents(amount string) (uint64, error) {
	//convert string to float
	//catch errors if number can not be converted
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0, err
	}

	//multiply float to get amount in cents
	//may return value short one penny but with .99999 fraction of a cent, i.e.: 32.55 -> 3254.9999999999995
	//or with additional penny but with fractions of a cent, i.e: 32.52 -> 3252.0000000000005
	amountFloatCents := amountFloat * 100

	//get rid of strange cents
	//add half a cent to number to "jump" to next cent if float amount was .99999 (if amount was .000005 this will not "jump" to the next cent)
	//then round down to the nearest whole number (removing the .599999 or .5000005)
	amountFloatCentsRounded := math.Floor(amountFloatCents + 0.5)

	//convert float to uint64
	//because thats what Stripe wants to process charges
	amountIntCents := uint64(amountFloatCentsRounded)

	//done
	return amountIntCents, nil
}

//ExtractDataFromCharge pulls out the fields of data we want from a stripe charge object
//we only need certain info from the stripe charge object, this pulls the needed fields out
//also does some formating for using the data in the gui
func ExtractDataFromCharge(chg *stripe.Charge) (data ChargeData) {
	//charge info
	id := chg.ID
	amountInt := chg.Amount
	captured := chg.Captured
	capturedStr := strconv.FormatBool(captured)
	timestamp := chg.Created

	//skip the rest of this if captured is false
	//this means the charge was not processed
	//for example: the card was declined
	if captured == false {
		return
	}

	//metadata
	meta := chg.Metadata
	customerName := meta["customer_name"]
	customerID := meta["customer_id"]
	invoice := meta["invoice_num"]
	po := meta["po_num"]
	username := meta["processed_by"]

	autoCharge, _ := strconv.ParseBool(meta["auto_charge"])
	autoChargeReferrer := meta["auto_charge_referrer"]
	autoChargeReason := meta["auto_charge_reason"]

	//customer info
	customer := chg.Customer
	j, _ := json.Marshal(customer)
	customer.UnmarshalJSON(j)
	stripeCustID := customer.ID

	//card info
	source := chg.Source
	j2, _ := json.Marshal(source)
	source.UnmarshalJSON(j2)
	card := source.Card
	cardholder := card.Name
	expMonth := strconv.FormatInt(int64(card.ExpMonth), 10)
	expYear := strconv.FormatInt(int64(card.ExpYear), 10)
	exp := expMonth + "/" + expYear
	last4 := card.Last4
	cardBrand := string(card.Brand)

	//convert amount to dollars
	amountDollars := strconv.FormatFloat((float64(amountInt) / 100), 'f', 2, 64)

	//convert timetamp to datetime
	datetime := time.Unix(timestamp, 0).Format("2006-01-02T15:04:05.000Z")

	//build data struct to return
	data = ChargeData{
		ID:            id,
		AmountCents:   amountInt,
		AmountDollars: amountDollars,
		Captured:      captured,
		CapturedStr:   capturedStr,
		Timestamp:     datetime,
		Invoice:       invoice,
		Po:            po,
		StripeCustID:  stripeCustID,
		Customer:      customerName,
		CustomerID:    customerID,
		User:          username,
		Cardholder:    cardholder,
		LastFour:      last4,
		Expiration:    exp,
		CardBrand:     cardBrand,

		AutoCharge:         autoCharge,
		AutoChargeReferrer: autoChargeReferrer,
		AutoChargeReason:   autoChargeReason,
	}

	return
}

//ExtractRefundsFromEvents pulls out the data for each refund from the list of events and formats the data as needed
//stripe does not allow easy retrieving of refund data like charge data
//have to search in the "history" aka event log for refunds
//then have to parse the data since the data is in json format and there is no easy way to convert the data to a struct
//note: there can be many refunds for a single charge (that total less than or equal to the total amount charged)
func ExtractRefundsFromEvents(eventList *event.Iter) (r []RefundData) {
	//loop through each refund event
	//each event is a charge
	//each charge can have one or more refunds
	for eventList.Next() {
		event := eventList.Event()
		charge := event.Data.Object

		//get charge data
		//*must* use map[string]interface and then type assert to .(string)...go throws errors otherwise
		card := charge["source"].(map[string]interface{})
		lastFour := card["last4"].(string)
		expMonth := strconv.FormatInt(int64(card["exp_month"].(float64)), 10)
		expYear := strconv.FormatInt(int64(card["exp_year"].(float64)), 10)
		expiration := expMonth + "/" + expYear

		//charge meta data
		meta := charge["metadata"].(map[string]interface{})
		custName := meta["customer_name"].(string)
		invoice := meta["invoice_num"].(string)

		//get refund data
		//have to check for "null" fields
		refundData := charge["refunds"].(map[string]interface{})
		refundList := refundData["data"].([]interface{})
		for _, v := range refundList {
			refund := v.(map[string]interface{})
			refundedAmountInt := refund["amount"].(float64)
			refundedTimestamp := refund["created"].(float64)

			refundReason := "unknown"
			if refund["reason"] != nil {
				refundReason = refund["reason"].(string)
			}

			refundedBy := "unknown"
			if refund["metadata"] != nil {
				rdMeta := refund["metadata"].(map[string]interface{})
				if rdMeta["processed_by"] != nil {
					refundedBy = rdMeta["processed_by"].(string)
				}
			}

			//get refunded amount in dollars
			refundedDollars := strconv.FormatFloat((refundedAmountInt / 100), 'f', 2, 64)

			//convert timestamp to datetime
			datetime := time.Unix(int64(refundedTimestamp), 0).Format("2006-01-02T15:04:05.000Z")

			//build struct to build template with
			rr := RefundData{
				Refunded:      true,
				AmountCents:   int64(refundedAmountInt),
				AmountDollars: refundedDollars,
				Timestamp:     datetime,
				Invoice:       invoice,
				LastFour:      lastFour,
				Expiration:    expiration,
				Customer:      custName,
				User:          refundedBy,
				Reason:        refundReason,
			}

			r = append(r, rr)
		}
	}

	//return list of refunds
	return
}
