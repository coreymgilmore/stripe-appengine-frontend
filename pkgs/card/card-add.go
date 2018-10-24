package card

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/memcacheutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/timestamps"
	"github.com/stripe/stripe-go"
)

//Add saves a new card the the datastore
//This is done by validating the inputs, sending the card token to Stripe, and saving
//the customer ID from Stripe to the datastore (along with some other info).
//The card token was generated client side by stripe.js.  Stripe.js takes the card
//number, expiration, and security code and sends it to Stripe.  It returns a token
//for us to use.  This makes it so we never "touch" or save the actually card information.
//The customer ID that we get back from Stripe is used to process charges in the future.
func Add(w http.ResponseWriter, r *http.Request) {
	//get form values
	customerID := r.FormValue("customerId")     //a unique key for the card, not the datastore id or stripe customer id
	customerName := r.FormValue("customerName") //user provided, could be company name/client name/may be same as cardholder
	cardholder := r.FormValue("cardholder")     //name on card as it appears
	cardToken := r.FormValue("cardToken")       //from stripe.js
	cardExp := r.FormValue("cardExp")           //from stripe.js, not from html input
	cardLast4 := r.FormValue("cardLast4")       //from stripe.js, not from html input

	//make sure all form values were given
	if len(customerName) == 0 {
		output.Error(errMissingCustomerName, "You did not provide the customer's name.", w, r)
		return
	}
	if len(cardholder) == 0 {
		output.Error(errMissingCustomerName, "You did not provide the cardholer's name.", w, r)
		return
	}
	if len(cardToken) == 0 {
		output.Error(errMissingCardToken, "A serious error occured; the card token is missing. Please refresh the page and try again.", w, r)
		return
	}
	if len(cardExp) == 0 {
		output.Error(errMissingExpiration, "The card's expiration date is missing from Stripe. Please refresh the page and try again.", w, r)
		return
	}
	if len(cardLast4) == 0 {
		output.Error(errMissingLast4, "The card's last four digits are missing from Stripe. Please refresh the page and try again.", w, r)
		return
	}

	//get context
	c := r.Context(r)

	//need to adjust deadline in case stripe takes longer than 5 seconds
	//default timeout for a urlfetch is 5 seconds
	//sometimes adding a card through stripe api takes longer
	//calls seems to take roughly 2 seconds normally with a few near 5 seconds (normal urlfetch deadline)
	//the call might still complete via stripe but appengine will return to the gui that it failed
	//10 seconds is a bit over generous but covers even really strange senarios
	c, cancelFunc := context.WithTimeout(c, 10*time.Second)
	defer cancelFunc()

	//if customerID was given, make sure it is unique
	//this id should be unique in the user's company's crm
	//the customerID is used to autofill the charge card panel when performing the api-like semi-automated charges
	if len(customerID) != 0 {
		_, err := FindByCustomerID(c, customerID)
		if err == nil {
			//customer already exists
			output.Error(errCustIDAlreadyExists, "This customer ID is already in use. Please double check your records or remove the customer with this customer ID first.", w, r)
			return
		} else if err != errCustomerNotFound {
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
		var errorErr error
		errorMsg := ""

		switch err.(type) {
		default:
			errorErr = errors.New("unknown error while adding card")
			errorMsg = "There was an error adding this card.  Please contact the administrator."
			break

		case *stripe.Error:
			stripeError := err.(*stripe.Error)
			stripeErrorErr := stripeError.Err
			stripeErrorMsg := stripeError.Msg
			log.Println("card.Add", stripeError)

			errorErr = stripeErrorErr
			errorMsg = stripeErrorMsg

			break

		case *url.Error:
			urlError := err.(*url.Error)
			urlErrorErr := urlError.Err
			log.Println("card.Add", urlError)

			errorErr = urlErrorErr
			errorMsg = "A url error occured (" + urlError.Error() + "). Contact the administrator to diagnose this issue."
			break
		}

		output.Error(errorErr, errorMsg, w, r)
		return
	}

	//get username of logged in user
	//used for tracking who added a card, just for diagnostics
	username := sessionutils.GetUsername(r)

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

	//delete list of cards in memcache
	//since a card was added, memcache is wrong
	//clients will retreive new list when refreshing page/app
	memcacheutils.Delete(c, listOfCardsKey)
	return
}

//createNewCustomerKey generates a new datastore key for saving a new customer/card
//Appengine's datastore does not generate this key automatically when an entity is saved.
func createNewCustomerKey() *datastore.Key {
	return datatore.IncompleteKey(datastoreKind, nil)
}

//save does the actual saving of a card to the datastore
//separate function to clean up code
func save(c context.Context, key *datastore.Key, customer CustomerDatastore) (*datastore.Key, error) {
	//connect to datastore
	client := datastoreutils.Client

	//save customer
	completeKey, err := client.Put(c, key, &customer)
	if err != nil {
		return key, err
	}

	//done
	return completeKey, nil
}
