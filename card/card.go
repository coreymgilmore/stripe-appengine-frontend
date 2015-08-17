package card 

import (
	"net/http"
	"encoding/json"
	"io/ioutil"
	"errors"

	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
)

const (
	//PATH TO STRIPE PRIVATE KEY FILE STORED IN TEXT
	//key is stored in file instead of code so it is easily changed
	STRIPE_PRIVATE_KEY_PATH = "secrets/stripe-private-key.txt"
)

var (
	//global store for the stripe key used for transactions
	stripePrivateKey = ""

	//save errors from Init() for use when checking if private key was read correctly
	initError error

	//error when checking if stripe private key file was empty
	ErrStripeKeyTooShort = errors.New("The Stripe private key ('stripe-private-key.txt') file was empty. Please provide your Stripe private key.")
)

//CONSISTANT OBJECT FOR RETURNING DATA TO CLIENT
type returnObj struct {
	Ok 		bool
	Title 	string
	Msg 	string
	Data 	map[string]interface{}
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

//ADD A NEW CUSTOMER TO THE DATASTORE
//stripe created a card token that can only be used once
//need to create a stripe customer to charge many times
//create the customer and save the stripe customer token along with the customer id and customer name
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
		returnResults(false, "errMissingCustomername", "You did not provide a customer name.", nil, w)
		return
	}
	if len(cardholder) == 0 {
		returnResults(false, "errMissingCardholder", "You did not provide the name of the cardholder.", nil, w)
		return
	}
	if len(cardToken) == 0 {
		returnResults(false, "errMissingCardToken", "A serious error occured. Please contact an administrator.", nil, w)
		return
	}
	if len(cardExp) == 0 {
		returnResults(false, "errMissingCardExp", "The card's expiration is missing.", nil, w)
		return
	}
	if len(cardLast4) == 0 {
		returnResults(false, "errMissingCardLast4", "The card's last four digits are missing.", nil, w)
		return
	}

	//create the stripe customer
	stripe.Key = stripePrivateKey
	stripe.SetHTTPClient(urlfetch.Client(appengine.NewContext(r)))

	custParams := &stripe.CustomerParams{
		Desc: 	customerId,
	}
	custParams.SetSource(cardToken)

	cust, err := customer.New(custParams)

	e := make(map[string]interface{})
	e["error"] = err
	e["cust"] = cust 

	returnResults(true, "asdf", "asdfsadfaf", e, w)
	return

}





//**********************************************************************
//FUNCS

//RETURN RESULTS TO CLIENT
func returnResults (ok bool, title, msg string, data map[string]interface{}, w http.ResponseWriter) {
	obj := returnObj{
		Ok: 		ok,
		Title: 		title,
		Msg: 		msg,
		Data: 		data,
	}

	//convert to json
	j, _ := json.Marshal(obj)

	//set http status code based on 'ok'
	if ok {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}

	//send back json
	w.Write(j)
	return
}

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
