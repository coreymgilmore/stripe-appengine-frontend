package card

import (
	"net/http"
	"strconv"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/stripe/stripe-go"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
)

//RemoveAPI removes a card from the datastore, memcache, and stripe
//This removes a card based upon the datastore ID.  This ID is tied into
//one Stripe customer and one card.
func RemoveAPI(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreID := r.FormValue("customerId")

	//make sure an id was given
	if len(datastoreID) == 0 {
		output.Error(errMissingInput, "A customer's datastore ID must be given but was missing. This value is different from your \"Customer ID\" and should have been submitted automatically.", w, r)
		return
	}

	//remove the card
	err := Remove(datastoreID, r)
	if err != nil {
		output.Error(err, "There was an error while trying to delete this customer. Please try again.", w, r)
		return
	}

	//done
	output.Success("removeCustomer", nil, w)
	return
}

//Remove does the actual removal of the card
func Remove(datastoreID string, r *http.Request) error {
	//convert to int
	datastoreIDInt, _ := strconv.ParseInt(datastoreID, 10, 64)

	//init stripe
	c := appengine.NewContext(r)
	sc := createAppengineStripeClient(c)

	//delete customer on stripe
	custData, err := findByDatastoreID(c, datastoreIDInt)
	if err != nil {
		return err
	}
	stripeCustID := custData.StripeCustomerToken
	sc.Customers.Del(stripeCustID, &stripe.CustomerParams{})

	//delete customer from datastore
	completeKey := getCustomerKeyFromID(c, datastoreIDInt)
	err = datastore.Delete(c, completeKey)
	if err != nil {
		return err
	}

	//delete customer from memcache
	//delete list of cards in memcache since this list is now stale
	//all memcache.Delete operations are listed first so error handling doesn't return
	//if one fails...each call does not depend on another so this is safe
	//obviously, if the card is not in the cache it cannot be removed
	err1 := memcache.Delete(c, datastoreID)
	err2 := memcache.Delete(c, custData.CustomerID)
	err3 := memcache.Delete(c, listOfCardsKey)
	if err1 != nil && err1 != memcache.ErrCacheMiss {
		return err1
	}
	if err2 != nil && err2 != memcache.ErrCacheMiss {
		return err2
	}
	if err3 != nil && err3 != memcache.ErrCacheMiss {
		return err3
	}

	//customer removed
	return nil
}
