package card

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sqliteutils"
	"google.golang.org/api/iterator"

	"cloud.google.com/go/datastore"
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
	c := r.Context()
	err := remove(c, datastoreID)
	if err != nil {
		output.Error(err, "There was an error while trying to delete this customer. Please try again.", w)
		return
	}

	//done
	output.Success("removeCustomer", nil, w)
	return
}

//remove does the actual removal of the card
func remove(ctx context.Context, datastoreID string) error {
	//convert to int
	datastoreIDInt, _ := strconv.ParseInt(datastoreID, 10, 64)

	//init stripe
	sc := CreateStripeClient(ctx)

	//delete customer on stripe
	custData, err := findByDatastoreID(ctx, datastoreIDInt)
	if err != nil {
		return err
	}
	stripeCustID := custData.StripeCustomerToken
	sc.Customers.Del(stripeCustID, &stripe.CustomerParams{})

	//use correct db
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		q := `
			DELETE FROM ` + sqliteutils.TableCards + ` 
			WHERE ID = ?
		`
		stmt, err := c.Prepare(q)
		if err != nil {
			return err
		}

		_, err = stmt.Exec(datastoreID)
		if err != nil {
			return err
		}

	} else {
		//delete customer from datastore
		client, err := datastoreutils.Connect(ctx)
		if err != nil {
			return err
		}

		completeKey := datastoreutils.GetKeyFromID(datastoreutils.EntityCards, datastoreIDInt)
		err = client.Delete(ctx, completeKey)
		if err != nil {
			return err
		}
	}

	//customer removed
	return nil
}

//RemoveExpiredCards removes old cards
//This works by removing any card whose expiration is in in the prior past month.
//This is designed to run monthly as a cron task.
func RemoveExpiredCards(w http.ResponseWriter, r *http.Request) {
	if sqliteutils.Config.UseSQLite {
		//currently not removing expired cards when using sqlite
		return
	}

	//connect to datastore
	c := r.Context()
	client, err := datastoreutils.Connect(c)
	if err != nil {
		output.Error(err, "Could not connect to datastore", w)
		return
	}

	//get previous month as a 1 or 2 digit number
	now := time.Now()
	month := int(now.Month() - 1)
	year := now.Year()

	//build month and year into string as we store expiration dates in datastore
	monthYear := strconv.Itoa(month) + "/" + strconv.Itoa(year)

	//user can also pass in monthYear as a form value
	//useful for removing cards more than 1 year in the past
	fv := r.FormValue("monthYear")
	if fv != "" {
		monthYear = fv
	}

	log.Println("card.RemoveExpiredCards", "Removing expired cards for: ", monthYear)

	//query datastore
	//need customer name for logging and stripe token to remove card from stripe
	fields := []string{"CustomerName", "StripeCustomerToken"}
	q := datastore.NewQuery("card").Filter("CardExpiration =", monthYear).Project(fields...)

	//iterate through results
	//only results should be cards that expired last month
	cardsRemovedCount := 0
	i := client.Run(c, q)
	for {
		customer := CustomerDatastore{}
		key, err := i.Next(&customer)
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Println("card.RemoveExpiredCards: Could not retrieve customer data. ", err)
			return
		}

		//remove the card from the datastore and from stripe
		datastoreID := strconv.FormatInt(key.ID, 10)
		_ = datastoreID
		err = remove(c, datastoreID)
		if err != nil {
			log.Println("card.RemoveExpiredCards: Could not remove card.", customer.CustomerName, err)
			return
		}

		cardsRemovedCount++
	}

	return
}
