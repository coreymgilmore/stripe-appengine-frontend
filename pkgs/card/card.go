/*
Package card implements functionality to add, remove, charge, and issue refunds for credit cards
as well as view reports for lists of charges and refunds.

Charges are performed via Stripe. This package requires loading the Stripe private key for your
Stripe account when the app starts.  The private key should be in the app.yaml file as an
environmental variable.

When a card is added to the app, very minimal (expiration and last 4) of the card's data
is actually stored in App Engine. The card's information is sent to Stripe who then
returns an id for this card. When a charge is processed, this id is sent to Stripe and
Stripe looks up the card's information to charge it.

The information stored in App Engine is safe; no credit card number is stored.  The data is
just used to identify the card so users of the app know which card they are charging.

Datastore ID: the ID the entity in the App Engine Datastore.
Customer ID: the ID that the user provides that links the card to a company.  This is usually from a CRM.
Stripe ID: the ID stripe uses to process a charge.  Also known as the stripe customer token.
For each datastore ID, there should be one and only one customer ID and stripe ID.
For each customer ID, there can be many datastore IDs and stripe IDs; one for each card added.
For each stripe ID, there should be one and only one datastore ID and customer ID.
*/
package card

import (
	"context"
	"errors"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/urlfetch"
)

const (
	//stripeSecretKeyLength is the exact length of a stripe secret key
	//this is used to make sure a valid stripe secret key was provided
	stripeSecretKeyLength = 32

	//datastoreKind is the name of the "table" or "collection" where card data is stored
	//we store the name to the "kind" in a const for easy reference in other code
	datastoreKind = "card"

	//currency for transactions
	//this should be a simple change for other currencies, but you will need to change the "$" symbol elsewhere in this code base
	currency = "usd"

	//minCharge is the lowest charge the app will allow
	//Stripe takes $0.30 + 2.9% of transactions so it is not worth collecting a charge that will cost us more then we will make
	//this is in cents
	minCharge = 50
)

//stripeSecretKey is the private api key from stripe used to charge cards
//this is read from app.yaml during init() and when creating a stripe client to process cards
var stripeSecretKey string

//errStripeKeyInvalid is used to describe an error with getting the Stripe secret key from and app.yaml when the app loads
var errStripeKeyInvalid = errors.New("card: the stripe secret key in app.yaml is invalid")

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

//init reads the stripe private key into the app
//an error is thrown if this is missing b/c we need this value to process charges
func init() {
	secretKey := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))

	//save key to Stripe so we can charge cards and perform other actions
	stripe.Key = secretKey

	//save key to create stripe client later
	stripeSecretKey = secretKey

	//done
	return
}

//CheckInit makes sure the init() ran successfully by checking if the
//stripeSecretKey was loaded
func CheckInit() error {
	if len(stripeSecretKey) == 0 {
		return errStripeKeyInvalid
	}

	return nil
}

//GetAll retrieves the list of all cards in the datastore
//This only gets the datastore id and customer name.
//The data is pulled from the datastore and is returned as json
//to build the datalist drop down where the user can choose what customer
//to charge.
func GetAll(w http.ResponseWriter, r *http.Request) {
	//connect to datastore
	client := datastoreutils.Client

	//get list from datastore
	//only need to get entity keys and customer names which cuts down on datastore usage
	q := datastore.NewQuery(datastoreKind).Order("CustomerName").Project("CustomerName")
	var cards []CustomerDatastore
	c := r.Context()
	keys, err := client.GetAll(c, q, &cards)
	if err != nil {
		output.Error(err, "Error retrieving list of cards from datastore.", w, r)
		return
	}

	//build result
	//format data to show just datastore id and customer name
	//creates a map of structs
	var idAndNames []List
	for i, r := range cards {
		x := List{r.CustomerName, keys[i].IntID()}
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
	datstoreID, _ := strconv.ParseInt(r.FormValue("customerId"), 10, 64)

	//get customer card data
	c := r.Context()
	data, err := findByDatastoreID(c, datstoreID)
	if err != nil {
		output.Error(err, "Could not find this customer's data.", w, r)
		return
	}

	//return data to client
	output.Success("cardFound", data, w)
	return
}

//getCustomerKeyFromID gets the full datastore key from the datastore id
//ID is just numeric while key is a long string with the appengine
//app name, kind name, etc.
//Key is what is actually used to find entities in the datastore.
func getCustomerKeyFromID(id int64) *datastore.Key {
	return datastore.IDKey(datastoreKind, id, nil)
}

//findByDatastoreID retrieves a card's information by its datastore id
//This returns all the info on a card that is needed to build the ui.
func findByDatastoreID(c context.Context, datastoreID int64) (data CustomerDatastore, err error) {
	key := getCustomerKeyFromID(datastoreID)
	fields := []string{"CustomerId", "CustomerName", "Cardholder", "CardLast4", "CardExpiration", "StripeCustomerToken"}
	data, err = datastoreFindEntity(c, "__key__ =", key, fields)
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
	fields := []string{"CustomerName", "Cardholder", "CardLast4", "CardExpiration", "StripeCustomerToken"}
	data, err = datastoreFindEntity(c, "CustomerId =", customerID, fields)
	if err != nil {
		return
	}

	return
}

//datastoreFindEntity finds one entity in the datastore
//This function wraps around the datastore package to clean up the code.
//The project input is a string slice listing the column names we would like returned.
func datastoreFindEntity(c context.Context, filterField string, filterValue interface{}, project []string) (data CustomerDatastore, err error) {
	//connect to datastore
	client := datastoreutils.Client

	//query
	//using GetAll b/c this lets us filter.  Get can only look up by key
	q := datastore.NewQuery(datastoreKind).Filter(filterField, filterValue).Limit(1).Project(project...)
	_, err := client.GetAll(c, q, &data)
	if err != nil {
		return
	}

	//check if we found any results
	//pretty simple, check if data is set in variable
	if len(data) == 0 {
		return CustomerDatastore{}, errCustomerNotFound
	}

	//return the result
	return data[0], nil
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
	httpClient := urlfetch.Client(c)
	return client.New(stripeSecretKey, stripe.NewBackends(httpClient))
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
