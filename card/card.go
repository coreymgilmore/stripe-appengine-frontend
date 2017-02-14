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

The information stored in App Engine is safe...no credit card number is stored.  The data is
just used to identify the card so users of the app know which card they are charging.
*/

package card

import (
	"errors"
	"math"
	"memcacheutils"
	"net/http"
	"os"
	"output"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/urlfetch"
)

//stripeSecretKeyLength is the exact length of a stripe secret key
//this is used to make sure a valid stripe secret key was provided
const stripeSecretKeyLength = 32

//maxStatementDescriptorLength is the maximum length of the statement description
//this is dictated by Stripe
const maxStatementDescriptorLength = 22

//datastoreKind is the name of the "table" or "collection" where card data is stored
//we store the name to the "kind" in a const for easy reference in other code
const datastoreKind = "card"

//listOfCardsKey is the key name for storing the list of cards in memcache
//this key holds a value that is equal to the json of all cards we have stored in the datastore
//it is used to get the list of cards faster then having to query the datastore every time
const listOfCardsKey = "list-of-cards"

//currency for transactions
//this should be a simple change for other currencies, but you will need to change the "$" symbol elsewhere in this code base
const currency = "usd"

//minCharge is the lowest charge the app will allow
//Stripe takes $0.30 + 2.9% of transactions so it is not worth collecting a charge that will cost us more then we will make
const minCharge = 50

//stripeStatementDescriptor is the description for your company shown on credit card statements
//this value is read in from environmental variable defined in app.yaml in init()
//but we need it to be accessible outside of init()
var stripeStatementDescriptor string

//stripeSecretKey is the private api key from stripe used to charge cards
//this is read in during init() and when creating a stripe client to process cards
var stripeSecretKey string

//init func errors
//since init() cannot return errors, we check for errors upon the app starting up
var (
	initError               error
	ErrStripeKeyInvalid     = errors.New("Card: The Stripe secret key you provided is invalid. Provide a valid Stripe secret key in app.yaml.")
	ErrStatementDescMissing = errors.New("Card: You did not provide a statement descriptor. Provide one in app.yaml.")
)

//other errors
var (
	ErrMissingCustomerName  = errors.New("Card: missingCustomerName")
	ErrMissingCardholerName = errors.New("Card: missingCardholderName")
	ErrMissingCardToken     = errors.New("Card: missingCardToken")
	ErrMissingExpiration    = errors.New("Card: missingExpiration")
	ErrMissingLast4         = errors.New("Card: missingLast4CardDigits")
	ErrStripe               = errors.New("Card: stripeError")
	ErrMissingInput         = errors.New("Card: missingInput")
	ErrChargeAmountTooLow   = errors.New("Card: amountLessThanMinCharge")
	ErrCustomerNotFound     = errors.New("Card: customerNotFound")
	ErrCustIDAlreadyExists  = errors.New("Card: customerIdAlreadyExists")
)

//init reads the private key and statement descriptor environmental varibales into the app
//the values of these files are saved for use in other parts of this app
//an error is thrown if either of these files is missing as they are both required for the app to work
func init() {
	//stripe private key
	secretKey := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
	if len(secretKey) != stripeSecretKeyLength {
		initError = ErrStripeKeyInvalid

	}

	//save key to Stripe so we can charge cards and perform other actions
	stripe.Key = secretKey

	//save key to create stripe client later
	stripeSecretKey = secretKey

	//statement descriptor
	stmtDesc := strings.TrimSpace(os.Getenv("STATEMENT_DESCRIPTOR"))
	if len(stmtDesc) == 0 {
		initError = ErrStatementDescMissing
		return
	}

	//trim to max length if needed
	if len(stmtDesc) > maxStatementDescriptorLength {
		stmtDesc = stmtDesc[:maxStatementDescriptorLength]
	}

	//Save description to variable for use when charging
	stripeStatementDescriptor = stmtDesc

	//done
	return
}

//CheckInit makes sure init() completed successfully since init() cannot return errors
func CheckInit() error {
	if initError != nil {
		return initError
	}

	return nil
}

//GetAll retrieves the list of all cards in the datastore (datastore id and customer name only)
//the data is pulled from memcache or the datastore
//the data is returned as json to populate the datalist in the html ui
func GetAll(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	//check if list of cards is in memcache
	var result []CardList
	_, err := memcache.Gob.Get(c, listOfCardsKey, &result)
	if err == nil {
		output.Success("cardlist-cached", result, w)
		return
	}

	//list of cards not found in memcache
	//get list from datastore
	//only need to get entity keys and customer names which cuts down on datastore usage
	//save the list to memcache for faster retrieval next time
	if err == memcache.ErrCacheMiss {
		q := datastore.NewQuery(datastoreKind).Order("CustomerName").Project("CustomerName")
		var cards []CustomerDatastore
		keys, err := q.GetAll(c, &cards)
		if err != nil {
			output.Error(err, "Error retrieving list of cards from datastore.", w, r)
			return
		}

		//build result
		//format data to show just datastore id and customer name
		//creates a map of structs
		var idAndNames []CardList
		for i, r := range cards {
			x := CardList{r.CustomerName, keys[i].IntID()}
			idAndNames = append(idAndNames, x)
		}

		//save list of cards to memcache
		//ignore errors since we already got the data
		memcacheutils.Save(c, listOfCardsKey, idAndNames)

		//return data to client
		output.Success("cardList-datastore", idAndNames, w)
		return

	} else if err != nil {
		output.Error(err, "Unknown error retrieving list of cards.", w, r)
		return
	}

	return
}

//GetOne retrieves the full data for one card from the datastore
//this is used to fill in the "charge card" panel with identifying info on the card so the user car verify they are charging the correct card
func GetOne(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	//get form value
	datstoreID, _ := strconv.ParseInt(r.FormValue("customerId"), 10, 64)

	//get customer card data
	data, err := findByDatastoreID(c, datstoreID)
	if err != nil {
		output.Error(err, "Could not find this customer's data.", w, r)
		return
	}

	//return data to client
	output.Success("cardFound", data, w)
	return
}

//getCustomerKeyFromID gets the full datastore key from the id
//id is just numeric, key is a long string with the appengine app name, kind name, etc.
//key is what is actually used to find entities in the datastore
func getCustomerKeyFromID(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, datastoreKind, "", id, nil)
}

//findByDatastoreID retrieves a card's information by its datastore id
//this returns all the info on a card that is needed to build the ui
//first memcache is checked for the data, then the datastore
func findByDatastoreID(c context.Context, datastoreID int64) (CustomerDatastore, error) {
	//check for card in memcache
	var r CustomerDatastore
	datastoreIDStr := strconv.FormatInt(datastoreID, 10)
	_, err := memcache.Gob.Get(c, datastoreIDStr, &r)
	if err == nil {
		return r, nil
	}

	//card data not found in memcache
	//look up data in datastore
	//save card to memcache after it is found
	if err == memcache.ErrCacheMiss {
		key := getCustomerKeyFromID(c, datastoreID)
		fields := []string{"CustomerId", "CustomerName", "Cardholder", "CardLast4", "CardExpiration", "StripeCustomerToken"}
		data, err := datastoreFindOne(c, "__key__ =", key, fields)
		if err != nil {
			return data, err
		}

		//save to memcache
		//ignore errors since we already got the data
		memcacheutils.Save(c, datastoreIDStr, data)

		//done
		return data, nil

	}

	//most likely an error occured
	return CustomerDatastore{}, err
}

//FindByCustID retrieves a card's information by the unique id from a CRM system
//this id was provided when a card was saved
//this func is used when making api style request to semi-automate the charging of a card.
//first memcache is checked for the data, then the datastore
func FindByCustID(c context.Context, customerID string) (CustomerDatastore, error) {
	//check for card in memcache
	var r CustomerDatastore
	_, err := memcache.Gob.Get(c, customerID, &r)
	if err == nil {
		return r, nil
	}

	//card data not found in memcache
	//look up data in datastore
	//save card to memcache after it is found
	if err == memcache.ErrCacheMiss {
		//only getting the fields we need to show data in the charge card panel
		fields := []string{"CustomerName", "Cardholder", "CardLast4", "CardExpiration"}
		data, err := datastoreFindOne(c, "CustomerId =", customerID, fields)
		if err != nil {
			return data, err
		}

		//save to memcache
		//ignore errors since we already got the data
		memcacheutils.Save(c, customerID, data)

		//done
		return data, nil

	}

	//most likely an error occured
	return CustomerDatastore{}, err
}

//datastoreFindOne finds one entity in the datastore
//this function wraps around the datastore package to clean up the code
//project is a string slice listing the column names we would like returned. less fields is more efficient
func datastoreFindOne(c context.Context, filterField string, filterValue interface{}, project []string) (CustomerDatastore, error) {
	//query
	query := datastore.NewQuery(datastoreKind).Filter(filterField, filterValue).Limit(1).Project(project...)
	var result []CustomerDatastore
	_, err := query.GetAll(c, &result)
	if err != nil {
		return CustomerDatastore{}, err
	}

	//check if one result exists
	if len(result) == 0 {
		return CustomerDatastore{}, ErrCustomerNotFound
	}

	//return the single result
	return result[0], nil
}

//calcTzOffset takes a string value input of the hours from UTC and outputs a timezone offset usable in golang
//input is a number such as -4 for EST generated via JS:
//var d = new Date(); (d.getTimezoneOffset() / 60 * -1);
//the output is a string in the format "-0400"
//the output is used to construct a golang time.Time
func calcTzOffset(hoursToUTC string) string {
	//placeholder for output
	var tzOffset = ""

	//get hours as a float
	hoursFloat, _ := strconv.ParseFloat(hoursToUTC, 64)

	//check if hours is before or after UTC
	//negative numbers are behind UTC (aka EST is -4 hours behind UTC)
	//add leading symbol to tzOffset output
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
//stripe's api is accessed via http requests, need a way to make these requests
//urlfetch is the appengine way of making http requests
//this func returns an httpclient *per request* aka per request to this app
//otherwise one request could use another requests httpclient
//this is for app engine only since the golang http.DefaultClient is unavailable
func createAppengineStripeClient(c context.Context) *client.API {
	//create http client
	httpClient := urlfetch.Client(c)

	//returns "sc" stripe client
	return client.New(stripeSecretKey, stripe.NewBackends(httpClient))
}

//getAmountAsIntCents converts a dollar amount as a string into a cents integer value
//amounts typed in ui form are dollars as a string (12.56)
//need amounts as cents to process payments via Stripe (1256)
//need to make sure value does add or lose decimal places during type conversions due to conversions from floats to int and different precisions
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

	//round up to get whole number in cents
	//gets rid of .99999 fraction of a cent

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
