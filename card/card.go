/*
	This package deals with anything regarding a card.
	You can save a new card (creating a customer), remove a card, charge/refund cards, and get reports on charges.
*/

package card

import (
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"

	"chargeutils"
	"memcacheutils"
	"output"
	"users"
)

const (
	//PATH TO PRIVATE KEY AND STATEMENT DESCRIPTOR
	//stored in separate text files so they are easily changed without having to edit code
	//values are read into the app upon initializing
	STRIPE_PRIVATE_KEY_PATH    = "config/stripe-secret-key.txt"
	STRIPE_STATEMENT_DESC_PATH = "config/statement-descriptor.txt"

	DATASTORE_KIND        = "card"
	LIST_OF_CARDS_KEYNAME = "list-of-cards"
	CURRENCY              = "usd"
	MIN_CHARGE            = 50 //cents
)

var (
	stripePrivateKey          = ""
	stripeStatementDescriptor = ""
	initError                 error

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

//SAVING CUSTOMER TO DATASTORE
type CustomerDatastore struct {
	CustomerId          string `json:"customer_id"`
	CustomerName        string `json:"customer_name"`
	Cardholder          string `json:"cardholder_name"`
	CardExpiration      string `json:"card_expiration"`
	CardLast4           string `json:"card_last4"`
	StripeCustomerToken string `json:"-"`
	DatetimeCreated     string `json:"-"`
	AddedByUser         string `json:"added_by"`
}

//CONFIRMING CUSTOMER WAS SAVED
type confirmCustomer struct {
	CustomerName   string
	Cardholder     string
	CardExpiration string
	CardLast4      string
}

//CHARGE SUCCESSFUL, RETURN DATA TO CLIENT
type chargeSuccessful struct {
	CustomerName   string `json:"customer_name"`
	Cardholder     string `json:"cardholder_name"`
	CardExpiration string `json:"card_expiration"`
	CardLast4      string `json:"card_last4"`
	Amount         string `json:"amount"`
	Invoice        string `json:"invoice"`
	Po             string `json:"po"`
	Datetime       string `json:"datetime"`
	ChargeId       string `json:"charge_id"`
}

//FOR RETURNING JUST A LIST OF CARDS
//used to build autocomplete datalist in html
type CardList struct {
	CustomerName string `json:"customer_name"`
	Id           int64  `json:"id"`
}

//FOR BUILDING REPORTS
type reportData struct {
	UserData    users.User               `json:"user_data"`
	StartDate   time.Time                `json:"start_datetime"`
	EndDate     time.Time                `json:"end_datetime"`
	Charges     []chargeutils.Data       `json:"charges"`
	Refunds     []chargeutils.RefundData `json:"refunds"`
	TotalAmount string                   `json:"total_amount"`
	NumCharges  uint16                   `json:"num_charges"`
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
	stripePrivateKey = string(apikey)
	stripe.Key = stripePrivateKey

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
//returns list of datastore IDs and names for each customer
//entire list if cached since looking up the list of cards happens often
//used to build the autocomplete datalists for the list of customers when removing, charging, or viewing reports
func GetAll(w http.ResponseWriter, r *http.Request) {
	//check if list of cards are in memcache
	//send back results if they are found
	result := make([]CardList, 0, 50)
	c := appengine.NewContext(r)
	_, err := memcache.Gob.Get(c, LIST_OF_CARDS_KEYNAME, &result)
	if err == nil {
		output.Success("cardlist-cached", result, w)
		return
	}

	//list of cards not found in memcache
	//get list from datastore
	//only need to get entity keys and customer names: cuts down on datastore usage
	if err == memcache.ErrCacheMiss {
		q := datastore.NewQuery(DATASTORE_KIND).Order("CustomerName").Project("CustomerName")
		cards := make([]CustomerDatastore, 0, 50)
		keys, err := q.GetAll(c, &cards)
		if err != nil {
			output.Error(err, "Error retrieving list of cards from datastore.", w)
			return
		}

		//build result
		//format data by datastore id an associated customer name
		//creates a map of structs
		idAndNames := make([]CardList, 0, 50)
		for i, r := range cards {
			x := CardList{r.CustomerName, keys[i].IntID()}
			idAndNames = append(idAndNames, x)
		}

		//save list of cards to memcache
		//ignore errors since we already got results
		memcacheutils.Save(c, LIST_OF_CARDS_KEYNAME, idAndNames)

		//return data to client
		output.Success("cardList-datastore", idAndNames, w)
		return

	} else if err != nil {
		output.Error(err, "Unknown error retrieving list of cards.", w)
		return
	}

	return
}

//GET INFO ON ONE CARD
//returns the card data for a given customer's datastore id
//used to fill in charge card panel with a card's last four digits and expiration
func GetOne(w http.ResponseWriter, r *http.Request) {
	//get form value
	datastoreId := r.FormValue("customerId")
	datstoreIdInt, _ := strconv.ParseInt(datastoreId, 10, 64)

	//get customer card data
	c := appengine.NewContext(r)
	data, err := findByDatastoreId(c, datstoreIdInt)
	if err != nil {
		output.Error(err, "Could not find this customer's data.", w)
		return
	}

	//return data to client
	output.Success("cardFound", data, w)
	return
}

//**********************************************************************
//DATASTORE

//CREATE COMPLETE KEY FOR USER
//get the full complete key from just the ID of a key
func getCustomerKeyFromId(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, DATASTORE_KIND, "", id, nil)
}

//GET CARD DATA BY THE DATASTORE ID
//use the datastore intID as the id when displayed in htmls
func findByDatastoreId(c context.Context, datastoreId int64) (CustomerDatastore, error) {
	//find data in memcache
	//if it does exist, return the data
	//if not, find the data in the datastore and save the data to memcache
	var r CustomerDatastore
	datastoreIdStr := strconv.FormatInt(datastoreId, 10)
	_, err := memcache.Gob.Get(c, datastoreIdStr, &r)
	if err == nil {
		return r, nil

	} else if err == memcache.ErrCacheMiss {
		//look up data in datastore
		key := getCustomerKeyFromId(c, datastoreId)
		data, err := datastoreFindOne(c, "__key__ =", key, []string{"CustomerName", "Cardholder", "CardLast4", "CardExpiration", "StripeCustomerToken"})
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
func FindByCustId(c context.Context, customerId string) (CustomerDatastore, error) {
	//find data in memcache
	//if it does exist, return the data
	//if not, find the data in the datastore and save the data to memcache
	var r CustomerDatastore
	_, err := memcache.Gob.Get(c, customerId, &r)
	if err == nil {
		return r, nil

	} else if err == memcache.ErrCacheMiss {
		//look up data in datastore
		//only getting the fields we need to show some simple data in the charge card panel
		data, err := datastoreFindOne(c, "CustomerId =", customerId, []string{"CustomerName", "Cardholder", "CardLast4", "CardExpiration"})
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
//project is a map of strings and each string is a field of data in an entity to get
//only the fields listed in project will be returned
//less fields is more efficient
func datastoreFindOne(c context.Context, filterField string, filterValue interface{}, project []string) (CustomerDatastore, error) {
	q := datastore.NewQuery(DATASTORE_KIND).Filter(filterField, filterValue).Limit(1).Project(project...)
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

//FORMAT STATEMENT DESCRIPTOR
//this is a max of 22 characters long
//just in case user specified a longer descritor in the config file
func formatStatementDescriptor() string {
	s := stripeStatementDescriptor

	if len(s) > 22 {
		return s[:22]
	}

	return s
}

//GET A GOLANG STYLE TIMEZONE OFFSET
//takes a string value input of the hours before or after UTC (-4 for EST for example)
//the input is generated via JS [var d = new Date(); (d.getTimezoneOffset() / 60 * -1);]
//outputs a string in the format: -0400
//output is used to contruct a golang time.Time from a string
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
func createAppengineStripeClient(c context.Context) *client.API {
	//create http client
	httpClient := urlfetch.Client(c)

	//returns "sc" stripe client
	return client.New(stripePrivateKey, stripe.NewBackends(httpClient))
}

//CONVERT AMOUNT FROM FORM VALUE INTO AN INT VALUE FOR STRING
//stripe requires unit64 for amount
//amount must be in cents (100, not $1.00)
//need to make sure value doesn't add or lose decimal places during type conversions
//returns error if amount cannot be converted
func getAmountAsIntCents(amount string) (uint64, error) {
	//convert string to float
	//catch errors if number can not be converted
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0, err
	}

	//multiply float to get amount in cents
	//may return value short one penny but with .99999 fraction of a cent
	//i.e.: 32.55 -> 3254.9999999999995
	amountFloatCents := amountFloat * 100

	//round up to get whole number in cents
	//gets rid of .99999 fraction of a cent
	amountFloatCentsRounded := math.Floor(amountFloatCents + 0.5)

	//convert float to uint64
	amountIntCents := uint64(amountFloatCentsRounded)

	//done
	return amountIntCents, nil
}
