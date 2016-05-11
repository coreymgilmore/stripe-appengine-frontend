/*
File card-remove.go implements functionality for removing a card from the app and stripe.
This is done when a card changes for a customer (we can only store one card per customer) or
the card on file is no longer being used.
*/

package card

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
	"net/http"
	"output"
	"strconv"
)

//Remove removes a card from the datastore, memcache, and stripe
func Remove(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreId := r.FormValue("customerId")
	datastoreIdInt, _ := strconv.ParseInt(datastoreId, 10, 64)

	//make sure an id was given
	if len(datastoreId) == 0 {
		output.Error(ErrMissingInput, "A customer's datastore ID must be given but was missing. This value is different from your \"Customer ID\" and should have been submitted automatically.", w, r)
		return
	}

	//init stripe
	c := appengine.NewContext(r)
	sc := createAppengineStripeClient(c)

	//delete customer on stripe
	custData, err := findByDatastoreId(c, datastoreIdInt)
	if err != nil {
		output.Error(err, "An error occured while trying to look up customer's Stripe information.", w, r)
	}
	stripeCustId := custData.StripeCustomerToken
	sc.Customers.Del(stripeCustId)

	//delete custome from datastore
	completeKey := getCustomerKeyFromId(c, datastoreIdInt)
	err = datastore.Delete(c, completeKey)
	if err != nil {
		output.Error(err, "There was an error while trying to delete this customer. Please try again.", w, r)
		return
	}

	//delete customer from memcache
	//delete list of cards in memcache since this list is now stale
	//all memcache.Delete operations are listed first so error handling doesn't return if one fails...each call does not depend on another so this is safe
	//obviously, if the card is not in the cache it cannot be removed
	err1 := memcache.Delete(c, datastoreId)
	err2 := memcache.Delete(c, custData.CustomerId)
	err3 := memcache.Delete(c, listOfCardsKey)
	if err1 != nil && err1 != memcache.ErrCacheMiss {
		output.Error(err1, "There was an error flushing this card's data from the cache (by datastore id). Please contact an administrator and have them flush the cache manually.", w, r)
		return
	}
	if err2 != nil && err2 != memcache.ErrCacheMiss {
		output.Error(err2, "There was an error flushing this card's data from the cache (by customer id). Please contact an administrator and have them flush the cache manually.", w, r)
		return
	}
	if err3 != nil && err3 != memcache.ErrCacheMiss {
		output.Error(err3, "There was an error flushing the cached list of cards.", w, r)
		return
	}

	//customer removed
	//return to client
	output.Success("removeCustomer", nil, w)
	return
}
