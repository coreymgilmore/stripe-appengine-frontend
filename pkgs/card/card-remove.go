package card

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sqliteutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/timestamps"
	"github.com/jmoiron/sqlx"
	"github.com/stripe/stripe-go"
	"google.golang.org/api/iterator"
)

//RemoveAPI removes a card from the datastore and stripe
//This removes a card based upon the datastore ID.  This ID is tied into
//one Stripe customer and one card.
func RemoveAPI(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreID, _ := strconv.ParseInt(r.FormValue("customerId"), 10, 64)

	//make sure an id was given
	if datastoreID == 0 {
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
func remove(ctx context.Context, datastoreID int64) error {
	//look up stripe id from database
	//look up stripe id for card
	custData, err := findByDatastoreID(ctx, datastoreID)
	if err != nil {
		return err
	}

	//delete customer on stripe
	//continue on stripe error so we still remove the card from our db
	err = removeFromStripe(ctx, custData.StripeCustomerToken)
	if err != nil {
		log.Println("card.remove - Could not remove card from stripe", err)
	}

	//use correct db
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		err := removeFromSQLite(c, datastoreID)
		if err != nil {
			return err
		}

	} else {
		client, err := datastoreutils.Connect(ctx)
		if err != nil {
			return err
		}

		completeKey := datastoreutils.GetKeyFromID(datastoreutils.EntityCards, datastoreID)
		err = removeFromDatastore(ctx, client, completeKey)
		if err != nil {
			return err
		}
	}

	//customer removed
	return nil
}

//removeFromStripe removed a card/customer from Stripe
func removeFromStripe(ctx context.Context, stripeCustomerID string) error {
	//init stripe
	sc := CreateStripeClient(ctx)

	//delete the card
	sc.Customers.Del(stripeCustomerID, &stripe.CustomerParams{})
	return nil
}

//removeFromSQLite removes a card/customer from the SQLite db
func removeFromSQLite(c *sqlx.DB, id int64) error {
	q := `
		DELETE FROM ` + sqliteutils.TableCards + ` 
		WHERE ID = ?
	`
	stmt, err := c.Prepare(q)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(id)
	return err
}

//removeFromDatastore removes a card/customer from cloud datastore
func removeFromDatastore(ctx context.Context, datastoreClient *datastore.Client, datastoreKey *datastore.Key) error {
	err := datastoreClient.Delete(ctx, datastoreKey)
	return err
}

//RemoveExpiredCards removes old cards
//This works by looking up cards whose expiration is a given month/year string.  Unfortunately
//this means that if this func doesn't run or encounters an error, cards older than the
//month/year will not be removed.  However, they will eventually get taken care of by
//RemoveUnusedCards.
//We do a "select" then a "delete" because we need the Stripe information to remove the
//card from Stripe.
//This is designed to be run monthly as a cron task.
func RemoveExpiredCards(w http.ResponseWriter, r *http.Request) {
	//get previous month as a 1 or 2 digit number
	now := time.Now()
	month := int(now.Month() - 1)
	year := now.Year()

	//build month and year into string as we store expiration dates in db
	monthYear := strconv.Itoa(month) + "/" + strconv.Itoa(year)

	//user can also pass in monthYear as a form value
	//useful for removing cards more than 1 year in the past
	fv := r.FormValue("monthYear")
	if fv != "" {
		monthYear = fv
	}

	log.Println("card.RemoveExpiredCards - Removing expired cards for: ", monthYear)

	//use correct db
	ctx := r.Context()
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		q := `
			SELECT 
				ID,
				StripeCustomerToken
			FROM ` + sqliteutils.TableCards + ` 
			WHERE CardExpiration = ?
		`

		potentialOldCards := []CustomerDatastore{}
		err := c.Select(&potentialOldCards, q, monthYear)
		if err != nil {
			log.Println("card.RemoveExpiredCards - Could not get list of old cards 2", err)
			return
		}

		//iterate through each card, removing each from Stripe and the db
		for _, p := range potentialOldCards {
			err := removeFromStripe(ctx, p.StripeCustomerToken)
			if err != nil {
				log.Println("card.RemoveExpiredCards - Could not remove card from Stripe with ID", p.StripeCustomerToken, err)
				return
			}

			err = removeFromSQLite(c, p.ID)
			if err != nil {
				log.Println("card.RemoveExpiredCards - Could not remove card from database with ID", p.ID, err)
				return
			}
		}

	} else {
		//connect to datastore
		c := r.Context()
		client, err := datastoreutils.Connect(c)
		if err != nil {
			log.Println("card.RemoveExpiredCards - Could not connect to datastore", err)
			return
		}

		//query datastore
		//cant do keys only since we need Stripe token to remove card from stripe
		fields := []string{"StripeCustomerToken"}
		q := datastore.NewQuery(datastoreutils.EntityCards).Filter("CardExpiration =", monthYear).Project(fields...)
		//iterate through results
		//only results should be cards that expired last month
		i := client.Run(c, q)
		for {
			customer := CustomerDatastore{}
			key, err := i.Next(&customer)
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Println("card.RemoveExpiredCards - Could not retrieve customer data. ", err)
				return
			}

			//remove the card from the datastore and from stripe
			err = removeFromStripe(ctx, customer.StripeCustomerToken)
			if err != nil {
				log.Println("card.RemoveExpiredCards - Could not remove card from Stripe with ID", customer.StripeCustomerToken, err)
				return
			}

			err = removeFromDatastore(ctx, client, key)
			if err != nil {
				log.Println("card.RemoveExpiredCards - Could not remove card from datastore with ID", key.ID, err)
				return
			}
		}
	}

	log.Println("card.RemoveExpiredCards...done")
	return
}

//RemoveUnusedCards removes card that we haven't charged in over 1 year
//We remove old cards to keep the db, Stripe, and the GUI dropdown menu of available cards
//cleaner.
//This works by looking up cards whose LastUsedTimestamp is greater than 1 year ago.
//We do a "select" then a "delete" because we need the Stripe information to remove the
//card from Stripe.
//This is designed to be run monthly as a cron task.
func RemoveUnusedCards(w http.ResponseWriter, r *http.Request) {
	//timestampe for 1 year ago, utc
	minAgeTimestamp := time.Now().UTC().AddDate(-1, 0, 0).Unix()

	//user can also provide a timestamp to manually select a timerange newer than 1 year
	fv, _ := strconv.ParseInt(r.FormValue("ts"), 10, 64)
	if fv != 0 {
		minAgeTimestamp = fv
	}

	log.Println("card.RemoveUnusedCards - removing cards that haven't been used since", minAgeTimestamp)

	//use correct db
	ctx := r.Context()
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		q := `
			SELECT 
				ID,
				StripeCustomerToken
			FROM ` + sqliteutils.TableCards + ` 
			WHERE LastUsedTimestamp < ?
		`

		unusedCards := []CustomerDatastore{}
		err := c.Select(&unusedCards, q, minAgeTimestamp)
		if err != nil {
			log.Println("card.RemoveUnusedCards - Could not get list of unusued cards 2", err)
			return
		}

		//iterate through each card, removing each from Stripe and the db
		for _, p := range unusedCards {
			err := removeFromStripe(ctx, p.StripeCustomerToken)
			if err != nil {
				log.Println("card.RemoveUnusedCards - Could not remove card from Stripe with ID", p.StripeCustomerToken, err)
				return
			}

			err = removeFromSQLite(c, p.ID)
			if err != nil {
				log.Println("card.RemoveUnusedCards - Could not remove card from database with ID", p.ID, err)
				return
			}
		}

	} else {
		//connect to datastore
		c := r.Context()
		client, err := datastoreutils.Connect(c)
		if err != nil {
			log.Println("card.RemoveUnusedCards - couldn't connect to datastore", err)
			return
		}

		//query datastore
		//cant do keys only since we need Stripe token to remove card from stripe
		fields := []string{"StripeCustomerToken"}
		q := datastore.NewQuery(datastoreutils.EntityCards).Filter("LastUsedTimestamp <", minAgeTimestamp).Project(fields...)
		i := client.Run(c, q)
		for {
			customer := CustomerDatastore{}
			key, err := i.Next(&customer)
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Println("card.RemoveUnusedCards - couldn't look up card to remove", err)
				return
			}

			//ignore cards whose LastUsedTimestamp is zero
			//cards that were added to this app prior to the LastUsedTimestamp existing will always return zero
			//so instead, update the card's LastUsedTimestamp to now
			//have to get full info for the card
			err = client.Get(c, key, &customer)
			if err != nil {
				log.Println("card.RemoveUnusedCards - Error while updating a zero LastUsedTimestamp 1", err)
				return
			}
			customer.LastUsedTimestamp = timestamps.Unix()
			_, err = saveDatatore(ctx, key, customer)
			if err != nil {
				log.Println("card.RemoveUnusedCards - Error while updating a zero LastUsedTimestamp 2", err)
				return
			}

			//remove the card from the datastore and from stripe
			err = removeFromStripe(ctx, customer.StripeCustomerToken)
			if err != nil {
				log.Println("card.RemoveUnusedCards - Could not remove card from Stripe with ID", customer.StripeCustomerToken, err)
				return
			}

			//remove the old card
			err = client.Delete(c, key)
			if err != nil {
				log.Println("card.RemoveUnusedCards - couldn't note delete unused card", err)
				return
			}
		}
	}

	log.Println("card.RemoveUnusedCards...done")
	return
}
