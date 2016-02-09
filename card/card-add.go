/*
	This file is part of the card package.
	This specifically deals with adding a new card for future charges.
	Cards are added by first creating a Stripe Customer.  Then the card is assigned to that customer.
	Card data is not stored locally, all that is stored in App Engine is the Stripe Customer Token.
	This token is used to perform future charges.
	Only one card can be saved per customer.
*/

package card

import (
	"net/http"
	"strconv"

	"github.com/coreymgilmore/timestamps"
	"github.com/stripe/stripe-go"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"

	"memcacheutils"
	"output"
	"sessionutils"
)

//ADD A NEW CARD TO THE DATASTORE
//stripe created a card token with stripe.js that can only be used once
//need to create a stripe customer to charge many times
//create the customer and save the stripe customer token along with the customer id and customer name to datastore
//the customer name is used to look up the stripe customer token that is used to charge the card
func Add(w http.ResponseWriter, r *http.Request) {
	//get form values
	customerId := r.FormValue("customerId")     //a unique key, not the datastore id or stripe customer id
	customerName := r.FormValue("customerName") //user provided, could be company name/client name/may be same as cardholder
	cardholder := r.FormValue("cardholder")     //name on card as it appears
	cardToken := r.FormValue("cardToken")       //from stripejs
	cardExp := r.FormValue("cardExp")           //from stripejs, not from html input
	cardLast4 := r.FormValue("cardLast4")       //from stripejs, not from html input

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
		output.Error(ErrMissingCardToken, "A serious error occured; the card token is missing. Please refresh the page and try again.", w)
		return
	}
	if len(cardExp) == 0 {
		output.Error(ErrMissingExpiration, "The card's expiration date is missing from Stripe. Please refresh the page and try again.", w)
		return
	}
	if len(cardLast4) == 0 {
		output.Error(ErrMissingLast4, "The card's last four digits are missing from Stripe. Please refresh the page and try again.", w)
		return
	}

	//init context
	c := appengine.NewContext(r)

	//if customerId was given, make sure it is unique
	//this id should be unique in the user's company's crm
	//the customerId is used to autofill the charge card panel
	if len(customerId) != 0 {
		_, err := FindByCustId(c, customerId)
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

	//create the customer on stripe
	//assigns the card via the cardToken to this customer
	//this card is used when making charges to this customer
	custParams := &stripe.CustomerParams{Desc: customerName}
	custParams.SetSource(cardToken)
	cust, err := sc.Customers.New(custParams)
	if err != nil {
		stripeErr := err.(*stripe.Error)
		stripeErrMsg := stripeErr.Msg
		output.Error(ErrStripe, stripeErrMsg, w)
		return
	}

	//get username of logged in user
	//used for tracking who added a card
	session := sessionutils.Get(r)
	username := session.Values["username"].(string)

	//save customer & card data to datastore
	newCustKey := createNewCustomerKey(c)
	newCustomer := CustomerDatastore{
		CustomerId:          customerId,
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
		output.Error(err, "There was an error while saving this customer. Please try again.", w)
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

//**********************************************************************
//DATASTORE

//CREATE INCOMPLETE KEY
func createNewCustomerKey(c context.Context) *datastore.Key {
	return datastore.NewIncompleteKey(c, datastoreKind, nil)
}

//SAVE CARD
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
