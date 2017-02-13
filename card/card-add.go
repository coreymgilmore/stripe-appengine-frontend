/*
File card-add.go implements functionality to add a new card to the app.
*/

package card

import (
	"memcacheutils"
	"net/http"
	"output"
	"sessionutils"
	"strconv"
	"timestamps"

	"golang.org/x/net/context"

	"github.com/stripe/stripe-go"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

//Add adds a new card the the appengine datastore
//this is done by validating the provided inputs, sending the card token to stripe, and saving the data to the datastore
//the card token was generated client side by the stipe-js
//  this is done so the card number and security code is never sent to the server
//  the server has no way of "touching" the card number for security
//when the card token is sent to stripe, stripe generates a customer token which we store and use to process payments
func Add(w http.ResponseWriter, r *http.Request) {
	//get form values
	customerID := r.FormValue("customerId")     //a unique key, not the datastore id or stripe customer id
	customerName := r.FormValue("customerName") //user provided, could be company name/client name/may be same as cardholder
	cardholder := r.FormValue("cardholder")     //name on card as it appears
	cardToken := r.FormValue("cardToken")       //from stripejs
	cardExp := r.FormValue("cardExp")           //from stripejs, not from html input
	cardLast4 := r.FormValue("cardLast4")       //from stripejs, not from html input

	//make sure all form values were given
	if len(customerName) == 0 {
		output.Error(ErrMissingCustomerName, "You did not provide the customer's name.", w, r)
		return
	}
	if len(cardholder) == 0 {
		output.Error(ErrMissingCustomerName, "You did not provide the cardholer's name.", w, r)
		return
	}
	if len(cardToken) == 0 {
		output.Error(ErrMissingCardToken, "A serious error occured; the card token is missing. Please refresh the page and try again.", w, r)
		return
	}
	if len(cardExp) == 0 {
		output.Error(ErrMissingExpiration, "The card's expiration date is missing from Stripe. Please refresh the page and try again.", w, r)
		return
	}
	if len(cardLast4) == 0 {
		output.Error(ErrMissingLast4, "The card's last four digits are missing from Stripe. Please refresh the page and try again.", w, r)
		return
	}

	//context
	c := appengine.NewContext(r)

	//if customerID was given, make sure it is unique
	//this id should be unique in the user's company's crm
	//the customerID is used to autofill the charge card panel when performing the api-like semi-automated charges
	if len(customerID) != 0 {
		_, err := FindByCustID(c, customerID)
		if err == nil {
			//customer already exists
			output.Error(ErrCustIDAlreadyExists, "This customer ID is already in use. Please double check your records or remove the customer with this customer ID first.", w, r)
			return
		} else if err != ErrCustomerNotFound {
			output.Error(err, "An error occured while verifying this customer ID does not already exist. Please try again or leave the customer ID blank.", w, r)
			return
		}
	}

	//init stripe
	sc := createAppengineStripeClient(c)

	//create the customer on stripe
	//assigns the card via the cardToken to this customer
	//this card is used when making charges to this customer
	custParams := &stripe.CustomerParams{Desc: customerName}
	custParams.SetSource(cardToken)
	cust, err := sc.Customers.New(custParams)
	if err != nil {
		stripeErr := err.(*stripe.Error)
		stripeErrMsg := stripeErr.Msg
		output.Error(ErrStripe, stripeErrMsg, w, r)
		return
	}

	//get username of logged in user
	//used for tracking who added a card, just for diagnostics
	session := sessionutils.Get(r)
	username := session.Values["username"].(string)

	//save customer & card data to datastore
	newCustKey := createNewCustomerKey(c)
	newCustomer := CustomerDatastore{
		CustomerID:          customerID,
		CustomerName:        customerName,
		Cardholder:          cardholder,
		CardExpiration:      cardExp,
		CardLast4:           cardLast4,
		StripeCustomerToken: cust.ID,
		DatetimeCreated:     timestamps.ISO8601(),
		AddedByUser:         username,
	}
	_, err = save(c, newCustKey, newCustomer)
	if err != nil {
		output.Error(err, "There was an error while saving this customer. Please try again.", w, r)
		return
	}

	//customer saved
	//return to client
	output.Success("createCustomer", nil, w)

	//resave list of cards in memcache
	//since a card was added, memcache is stale
	//clients will retreive new list when refreshing page/app
	memcacheutils.Delete(c, listOfCardsKey)
	return
}

//createNewCustomerKey generates a new datastore key for saving a new customer/card
//appengine's datastore does not generate this key automatically when an entity is saved
func createNewCustomerKey(c context.Context) *datastore.Key {
	return datastore.NewIncompleteKey(c, datastoreKind, nil)
}

//save does the actual saving of a card to the datastore
//separate function to clean up code
func save(c context.Context, key *datastore.Key, customer CustomerDatastore) (*datastore.Key, error) {
	//save customer
	completeKey, err := datastore.Put(c, key, &customer)
	if err != nil {
		return key, err
	}

	//save customer to memcache
	//have to generate a memcache key b/c memcache keys must be strings
	mKey := strconv.FormatInt(completeKey.IntID(), 10)
	err = memcacheutils.Save(c, mKey, customer)
	if err != nil {
		return completeKey, err
	}

	//done
	return completeKey, nil
}
