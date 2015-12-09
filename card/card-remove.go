/*
	This file is part of the card package.
	This specifically deals with removing cards/customers from the datastore and Stripe.
	This is broken into a separate file for organization.
*/

package card

import (
	"net/http"
	"strconv"

	"google.golang.org/appengine"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	"output"
)

//REMOVE A CARD
//remove from memcache, datastore, and stripe
func Remove(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreId := r.FormValue("customerId")
	datastoreIdInt, _ := strconv.ParseInt(datastoreId, 10, 64)

	//make sure an id was given
	if len(datastoreId) == 0 {
		output.Error(ErrMissingInput, "A customer's datastore ID must be given but was missing. This value is different from your \"Customer ID\" and should have been submitted automatically.", w)
		return
	}

	//init stripe
	c := appengine.NewContext(r)
	sc := createAppengineStripeClient(c)

	//delete customer on stripe
	custData, err := findByDatastoreId(c, datastoreIdInt)
	if err != nil {
		output.Error(err, "An error occured while trying to look up customer's Stripe information.", w)
	}
	stripeCustId := custData.StripeCustomerToken
	sc.Customers.Del(stripeCustId)

	//delete custome from datastore
	completeKey := getCustomerKeyFromId(c, datastoreIdInt)
	err = datastore.Delete(c, completeKey)
	if err != nil {
		output.Error(err, "There was an error while trying to delete this customer. Please try again.", w)
		return
	}

	//delete customer from memcache
	//delete list of cards in memcache since this list is stale
	//all memcache.Delete operations are listed first so error handling doesn't return if one fails...each call does not depend on another so this is safe
	//obviously, if the card is not in the cache it cannot be removed
	
	//*****
	//bunch of err handling here to figure out why sometimes a card is not deleted from memcahce
	//user can remove card, but cannot add a card to the same Account ID (customerId)
	//for some reason the cache is not clearing this card's data that is being stored by customerID
	//even though the card is no longer in the datastore and a simple "flush cache" fixes the problem
	err1 := memcache.Delete(c, datastoreId)
	err2 := memcache.Delete(c, custData.CustomerId)
	err3 := memcache.Delete(c, LIST_OF_CARDS_KEYNAME)
	
	//*****
	//still getting errors when adding a card for a customer after just removing a card
	//aka the card for a customer changed
	//log out data
	log.Debugf(c, "Flush memcache by datastore id | ", err1)
	log.Debugf(c, "Flush memcache by customer id | ", err2)
	log.Debugf(c, "Flush memcache list of cards | ", err3)
	//*****


	if err1 != nil && err1 != memcache.ErrCacheMiss {
		output.Error(err1, "There was an error flushing this card's data from the cache (by datastore id). Please contact an administrator and have them flush the cache manually.", w)
		return
	}
	if err2 != nil && err2 != memcache.ErrCacheMiss {
		output.Error(err2, "There was an error flushing this card's data from the cache (by customer id). Please contact an administrator and have them flush the cache manually.", w)
		return
	}
	if err3 != nil && err3 != memcache.ErrCacheMiss {
		output.Error(err3, "There was an error flushing the cached list of cards.", w)
		return
	}

	//customer removed
	//return to client
	output.Success("removeCustomer", nil, w)
	return
}
