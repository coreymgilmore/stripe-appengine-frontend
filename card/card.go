package card 

import (
	"net/http"
	"io/ioutil"
	"errors"
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/urlfetch"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
	"output"
	"github.com/coreymgilmore/timestamps"
	"memcacheutils"
	"strconv"
)

const (
	//PATH TO STRIPE PRIVATE KEY FILE STORED IN TEXT
	//key is stored in file instead of code so it is easily changed
	STRIPE_PRIVATE_KEY_PATH = 	"secrets/stripe-secret-key.txt"

	DATASTORE_KIND = 			"card"
	LIST_OF_CARDS_KEYNAME = 	"list-of-cards"
)

var (
	//global store for the stripe key used for transactions
	stripePrivateKey = ""

	//save errors from Init() for use when checking if private key was read correctly
	initError error

	//error when checking if stripe private key file was empty
	ErrStripeKeyTooShort = errors.New("The Stripe private key ('stripe-secret-key.txt') file was empty. Please provide your Stripe secret key.")

	ErrMissingCustomerName = 	errors.New("missingCustomerName")
	ErrMissingCardholerName = 	errors.New("missingCardholderName")
	ErrMissingCardToken = 		errors.New("missingCardToken")
	ErrMissingExpiration = 		errors.New("missingExpiration")
	ErrMissingLast4 = 			errors.New("missingLast4CardDigits")
	ErrStripe =					errors.New("stripeError")
)

//SAVING CUSTOMER TO DATASTORE
type customerDatastore struct {
	CustomerId 			string
	CustomerName 		string
	Cardholder 			string
	CardExpiration 		string
	CardLast4 			string
	StripeCustomerToken string
	DatetimeCreated 	string
}

//CONFIRMING CUSTOMER WAS SAVED
type confirmCustomer struct{
	CustomerName 		string
	Cardholder 			string
	CardExpiration 		string
	CardLast4 			string	
}

type cardList struct {
	CustomerName 		string	`json:"customer_name"`
	Id 					int64 	`json:"id"`
}

//**********************************************************************
//INIT
//read stripe private key from file and save it to variable
func Init() error {
	apikey, err := ioutil.ReadFile(STRIPE_PRIVATE_KEY_PATH)
	if err != nil {
		initError = err
		return err
	}

	//save key to session
	stripePrivateKey = string(apikey)
	return nil
}

//**********************************************************************
//HANDLE HTTP REQUESTS

//ADD A NEW CARD TO THE DATASTORE
//stripe created a card token that can only be used once
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
	if len(cardExp) == 0 {
		output.Error(ErrMissingExpiration, "The card's expiration date is missing.", w)
		return
	}
	if len(cardLast4) == 0 {
		output.Error(ErrMissingLast4, "The card's last four digits are missing.", w)
		return
	}

	//create the stripe customer
	stripe.Key = stripePrivateKey
	stripe.SetHTTPClient(urlfetch.Client(appengine.NewContext(r)))
	custParams := &stripe.CustomerParams{Desc: 	customerName}
	custParams.SetSource(cardToken)
	cust, err := customer.New(custParams)
	if err != nil {
		stripeErr := 		err.(*stripe.Error)
		stripeErrMsg := 	stripeErr.Msg
		output.Error(ErrStripe, stripeErrMsg, w)
		return
	}

	//customer created on stripe
	//save data to datastore
	newCustomer := customerDatastore{
		CustomerId: 			customerId,
		CustomerName: 			customerName,
		Cardholder: 			cardholder,
		CardExpiration: 		cardExp,
		CardLast4: 				cardLast4,
		StripeCustomerToken: 	cust.ID,
		DatetimeCreated: 		timestamps.ISO8601(),
	}

	//generate new incomplete key
	c := 				appengine.NewContext(r)
	incompleteKey := 	createNewCustomerKey(c)

	//save
	//datastore and memcache
	_, err = 			save(c, incompleteKey, newCustomer)
	if err != nil {
		output.Error(err, "There was an error while saving this customer.", w)
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
	return
}

//GET LIST OF CARDS
func GetAll(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	//check if list of cards is saved in memcache
	result := make([]cardList, 0, 25)
	_, err := memcache.Gob.Get(c, LIST_OF_CARDS_KEYNAME, &result)
	if err == nil {
		output.Success("cardlist-cached", result, w)
		return
	}

	//look up list of cards from datastore
	if err == memcache.ErrCacheMiss {
		q := 			datastore.NewQuery(DATASTORE_KIND).Order("CustomerName").Project("CustomerName")
		cards := 		make([]customerDatastore, 0, 25)
		keys, err := 	q.GetAll(c, &cards)
		if err != nil {
			output.Error(err, "Error retrieving list of cards from datastore.", w)
			return
		}

		//build result
		idAndNames := make([]cardList, 0, 25)
		for i, r := range cards {
			x := cardList{
				CustomerName: 	r.CustomerName,
				Id: 			keys[i].IntID(),
			}

			idAndNames = append(idAndNames, x)
		}

		//save list of cards to memcache
		//ignore errors since we already got results
		memcacheutils.Save(c, LIST_OF_CARDS_KEYNAME, idAndNames)

		//return data to client
		output.Success("cardList", idAndNames, w)
		return
	
	} else if err != nil {
		output.Error(err, "Unknown error retrieving list of cards.", w)
		return
	}

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
	key := datastore.NewKey(c, DATASTORE_KIND, "", id, nil)
	return key
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
func save(c appengine.Context, key *datastore.Key, customer customerDatastore) (*datastore.Key, error) {
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
