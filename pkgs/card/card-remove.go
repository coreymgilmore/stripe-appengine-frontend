package card

import (
	"net/http"
	"strconv"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/stripe/stripe-go"
)

//RemoveAPI removes a card from the datastore and stripe
//This removes a card based upon the datastore ID.  This ID is tied into
//one Stripe customer and one card.
func RemoveAPI(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreID := r.FormValue("customerId")

	//make sure an id was given
	if len(datastoreID) == 0 {
		output.Error(errMissingInput, "A customer's datastore ID must be given but was missing. This value is different from your \"Customer ID\" and should have been submitted automatically.", w)
		return
	}

	//remove the card
	err := Remove(datastoreID, r)
	if err != nil {
		output.Error(err, "There was an error while trying to delete this customer. Please try again.", w)
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
	c := r.Context()
	sc := CreateStripeClient(c)

	//delete customer on stripe
	custData, err := findByDatastoreID(c, datastoreIDInt)
	if err != nil {
		return err
	}
	stripeCustID := custData.StripeCustomerToken
	sc.Customers.Del(stripeCustID, &stripe.CustomerParams{})

	//delete customer from datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return err
	}

	completeKey := datastoreutils.GetKeyFromID(datastoreutils.EntityCards, datastoreIDInt)
	err = client.Delete(c, completeKey)
	if err != nil {
		return err
	}

	//customer removed
	return nil
}
