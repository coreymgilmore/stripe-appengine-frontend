/*
Package card implements functionality to add, remove, charge, and issue refunds for credit cards
as well as view reports for lists of charges and refunds.

Charges are performed via Stripe. This package requires loading the Stripe private key for your
Stripe account when the app starts.  The private key should be in the /config folder with the
name "stripe-secret-key.txt"

There is also a requirement for another setup file, "statement-descriptor.txt" in the /config
folder that is used to show a note on card holder's credit card statements.

When a card is added to the app, none of the card's data is actually stored in App Engine. The
card's information is sent to Stripe who then returns an id for this card. When a charge is
processed, this id is sent to Stripe and Stripe looks up the card's information to charge it.
The information stored in App Engine is safe...no credit card number is stored.  The data is
just used to identify the Stripe ID so users of the app know which card they are charging.
*/

package card

import (
	"chargeutils"
	"errors"
	"io/ioutil"
	"math"
	"memcacheutils"
	"net/http"
	"output"
	"strconv"
	"strings"
	"time"
	"users"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/urlfetch"
)

const (
	//Path to Stripe private key file
	//this needs to be the "live" key to perform charges that actually collect money
	//otherwise the "test" key can be used but only with test card numbers
	//this is stored in a separate file so it is not mistakenly saved with version control since .gitignore ignores the /config directory
	stripePriateKeyPath = "config/stripe-secret-key.txt"

	//Path to the statement descriptor of charges made through this app
	//this is the short note that shows up on the card holder's statement
	//this is stored in a separate file outside of the code for easy changing
	stripeStatementDescPath = "config/statement-descriptor.txt"

	//The app engine datastore "kind" is similar to a "collection" or "table" in other dbs.
	datastoreKind = "card"

	//Define the key name for storing the list of cards in memcache
	//this key holds a value that is equal to the json of all cards we have stored in the datastore
	//it is used to get the list of cards faster then having to query the datastore every time
	listOfCardsKey = "list-of-cards"

	//Currency used for transactions
	currency = "usd"

	//Minimum charge the app will allow
	//Stripe takes $0.30 + 2.9% of transactions so it is not worth collecting a charge that will cost us more then we will make
	minCharge = 50

	//The max amount of characters that will show up on a statement
	//this is dictated by Stripe but via the card companies
	maxStatementDescriptorLength = 22
)

var (
	//Text from files gets read into these variables
	stripePrivateKey          = ""
	stripeStatementDescriptor = ""

	//For reporting errors upon app initialization
	initError error

	//Desctiption of errors
	ErrStripeKeyTooShort    = errors.New("The Stripe private key ('stripe-secret-key.txt') file was empty. Please provide your Stripe secret key.")
	ErrStatementDescMissing = errors.New("The statement descriptor ('statement-descriptor.txt') file was empty. Please provide a statement descriptor.")
	ErrMissingCustomerName  = errors.New("missingCustomerName")
	ErrMissingCardholerName = errors.New("missingCardholderName")
	ErrMissingCardToken     = errors.New("missingCardToken")
	ErrMissingExpiration    = errors.New("missingExpiration")
	ErrMissingLast4         = errors.New("missingLast4CardDigits")
	ErrStripe               = errors.New("stripeError")
	ErrMissingInput         = errors.New("missingInput")
	ErrChargeAmountTooLow   = errors.New("amountLessThanMinCharge")
	ErrCustomerNotFound     = errors.New("customerNotFound")
	ErrCustIdAlreadyExists  = errors.New("customerIdAlreadyExists")
)

//CustomerDatastore is the format for data being saved to the datastore when a new customer is added
//this data is also returned when looking up a customer
type CustomerDatastore struct {
	//The CRM id of the customer
	//used when performing semi-automated api style requests to this app
	//this should be unique for every customer but can be left blank
	CustomerId string `json:"customer_id"`

	//The name of the customer
	//usually this is the company an individual card holder works for
	CustomerName string `json:"customer_name"`

	//The name on the card
	Cardholder string `json:"cardholder_name"`

	//MM/YYYY
	//used to just show in the gui which card is on file
	CardExpiration string `json:"card_expiration"`

	//Used to just show in the gui which card is on file
	CardLast4 string `json:"card_last4"`

	//The id returned when a card is saved via Stripe
	//this id uniquely identifies this card for this customer
	StripeCustomerToken string `json:"-"`

	//When was this card added to the app
	DatetimeCreated string `json:"-"`

	//Which user of the app saved the card
	//mostly used for diagnostics in the cloud platform console
	AddedByUser string `json:"added_by"`
}

//chargeSuccessful is used to return data to the gui when a charge is processed
//this data shows which card was processed and some confirmation details.
type chargeSuccessful struct {
	CustomerName   string `json:"customer_name"`
	Cardholder     string `json:"cardholder_name"`
	CardExpiration string `json:"card_expiration"`
	CardLast4      string `json:"card_last4"`

	//The amount of the charge as a dollar amount string
	Amount string `json:"amount"`

	//The invoice number to reference for this charge
	Invoice string `json:"invoice"`

	//The po number to reference for this charge
	Po string `json:"po"`

	//When the charge was processed
	Datetime string `json:"datetime"`

	//The id returned by stripe for this charge
	//used to show a receipt if needed
	ChargeId string `json:"charge_id"`
}

//CardList is used to return the list of cards available to be charged to build the gui
//the list is filled into a datalist in html to form an autocomplete list
//the app user chooses a card from this list to remove or process a charge
type CardList struct {
	//Company name for the card
	CustomerName string `json:"customer_name"`

	//The App Engine datastore id of the card
	//This is what uniquely identifies the card in the app engine datatore so we can look up the stripe customer token to process a charge
	Id int64 `json:"id"`
}

//reportData is used to build the report UI
type reportData struct {
	//The data for the logged in user
	//so we can show/hide certain UI elements based on the user's access rights
	UserData users.User `json:"user_data"`

	//The datetime we are filtering for getting report data
	//to limit the days a report gets data for
	StartDate time.Time `json:"start_datetime"`
	EndDate   time.Time `json:"end_datetime"`

	//Data for each charge for the report
	//this is a bunch of "rows" from Stripe
	Charges []chargeutils.Data `json:"charges"`

	//Data for each refund for the report
	//similar to Charges above
	Refunds []chargeutils.RefundData `json:"refunds"`

	//The total amount of all charges within the report date range
	TotalAmount string `json:"total_amount"`

	//Number of charges within the report date range
	NumCharges uint16 `json:"num_charges"`
}

//Init reads the private key and statement descriptor text files into the app
//the values of these files are saved for use in other parts of this app
//an error is thrown if either of these files is missing as they are both required for the app to work
func Init() error {
	//Stripe private key
	apikey, err := ioutil.ReadFile(stripePriateKeyPath)
	if err != nil {
		initError = err
		return err
	}

	//Save key to Stripe
	//so we can charge cards and perform other actions
	//convert to a string since stripe requires the api key to be passed as a string
	//remove spaces since any whitespace will cause errors
	stripePrivateKey = strings.TrimSpace(string(apikey))
	stripe.Key = stripePrivateKey

	//Statement descriptor
	descriptor, err := ioutil.ReadFile(stripeStatementDescPath)
	if err != nil {
		initError = err
		return err
	}

	//Save description to variable for use when charging
	stripeStatementDescriptor = string(descriptor)

	return nil
}

//**********************************************************************
//HANDLE HTTP REQUESTS

//GetAll retrieves the list of all cards in the datastore (datastore id and customer name only)
//the data is pulled from memcache or the datastore
//the data is returned as json to populate the datalist in the html ui
func GetAll(w http.ResponseWriter, r *http.Request) {
	//check if list of cards is in memcache
	c := appengine.NewContext(r)
	result := make([]CardList, 0, 50)
	_, err := memcache.Gob.Get(c, listOfCardsKey, &result)
	if err == nil {
		output.Success("cardlist-cached", result, w)
		return
	}

	//list of cards not found in memcache
	//get list from datastore
	//only need to get entity keys and customer names: cuts down on datastore usage
	//save the list to memcache for faster retrieval next time
	if err == memcache.ErrCacheMiss {
		q := datastore.NewQuery(datastoreKind).Order("CustomerName").Project("CustomerName")
		cards := make([]CustomerDatastore, 0, 50)
		keys, err := q.GetAll(c, &cards)
		if err != nil {
			output.Error(err, "Error retrieving list of cards from datastore.", w, r)
			return
		}

		//build result
		//format data to show just datastore id and customer name
		//creates a map of structs
		idAndNames := make([]CardList, 0, 50)
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
	//get form value
	datastoreId := r.FormValue("customerId")
	datstoreIdInt, _ := strconv.ParseInt(datastoreId, 10, 64)

	//get customer card data
	c := appengine.NewContext(r)
	data, err := findByDatastoreId(c, datstoreIdInt)
	if err != nil {
		output.Error(err, "Could not find this customer's data.", w, r)
		return
	}

	//return data to client
	output.Success("cardFound", data, w)
	return
}

//**********************************************************************
//DATASTORE

//getCustomerKeyFromId gets the full datastore key from the id
//id is just numeric, key is a big string with the appengine name, kind name, etc.
//key is what is actually used to find entities in the datastore
func getCustomerKeyFromId(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, datastoreKind, "", id, nil)
}

//findByDatastoreId retrieves a card's information by its datastore id
//this returns all the info on a card that is needed to build the ui
//first memcache is checked for the data, then the datastore
func findByDatastoreId(c context.Context, datastoreId int64) (CustomerDatastore, error) {
	//check for card in memcache
	var r CustomerDatastore
	datastoreIdStr := strconv.FormatInt(datastoreId, 10)
	_, err := memcache.Gob.Get(c, datastoreIdStr, &r)
	if err == nil {
		return r, nil
	}

	//card data not found in memcache
	//look up data in datastore
	//save card to memcache after it is found
	if err == memcache.ErrCacheMiss {
		key := getCustomerKeyFromId(c, datastoreId)
		data, err := datastoreFindOne(c, "__key__ =", key, []string{"CustomerId", "CustomerName", "Cardholder", "CardLast4", "CardExpiration", "StripeCustomerToken"})
		if err != nil {
			return data, err
		}

		//save to memcache
		//ignore errors since we already got the data
		memcacheutils.Save(c, datastoreIdStr, data)

		//done
		return data, nil

	} else {
		return CustomerDatastore{}, err
	}

	return CustomerDatastore{}, err
}

//FindByCustId retrieves a card's information by the unique id from a CRM system
//this id was provided when a card was saved
//this func is used when making api style request to semi-automate the charging of a card.
//first memcache is checked for the data, then the datastore
func FindByCustId(c context.Context, customerId string) (CustomerDatastore, error) {
	//check for card in memcache
	var r CustomerDatastore
	_, err := memcache.Gob.Get(c, customerId, &r)
	if err == nil {
		return r, nil
	}

	//card data not found in memcache
	//look up data in datastore
	//save card to memcache after it is found
	if err == memcache.ErrCacheMiss {
		//only getting the fields we need to show data in the charge card panel
		data, err := datastoreFindOne(c, "CustomerId =", customerId, []string{"CustomerName", "Cardholder", "CardLast4", "CardExpiration"})
		if err != nil {
			return data, err
		}

		//save to memcache
		//ignore errors since we already got the data
		memcacheutils.Save(c, customerId, data)

		//done
		return data, nil

	} else {
		return CustomerDatastore{}, err
	}

	return CustomerDatastore{}, err
}

//datastoreFindOne finds one entity in the datastore
//this function wraps around the datastore package to clean up the code
//project is a string slice listing the column names we would like returned. less fields is more efficient
func datastoreFindOne(c context.Context, filterField string, filterValue interface{}, project []string) (CustomerDatastore, error) {
	q := datastore.NewQuery(datastoreKind).Filter(filterField, filterValue).Limit(1).Project(project...)
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

//**********************************************************************
//FUNCS

//CheckStripe is used to make sure that a Stripe private key was provided
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

//FORMAT STATEMENT DESCRIPTOR
//this is a max of 22 characters long
//just in case user specified a longer descriptor in the config file

//formatStatementDescripto trims the text from the statement descriptor file down to the max size allowed
func formatStatementDescriptor() string {
	s := stripeStatementDescriptor

	if len(s) > maxStatementDescriptorLength {
		return s[:maxStatementDescriptorLength]
	}

	return s
}

//calcTxOffset takes a string value input of the hours from UTC and outputs a timezone offset usable in golang
//input is a number such as -4 for EST generated via JS:
//  var d = new Date(); (d.getTimezoneOffset() / 60 * -1);
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
	return client.New(stripePrivateKey, stripe.NewBackends(httpClient))
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
