package cron

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/card"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

func RemoveExpiredCards(w http.ResponseWriter, r *http.Request) {
	//get context
	c := appengine.NewContext(r)

	//query datastore
	fields := []string{"CustomerId", "CustomerName", "CardExpiration", "StripeCustomerToken"}
	q := datastore.NewQuery("card").Project(fields...).Limit(10)

	//get current datetime
	//even though we only need date
	//do this outside of for loop so we don't redo it constantly
	now := time.Now()

	//iterate through results
	for t := q.Run(c); ; {
		var customer card.CustomerDatastore

		//get one customer result
		datastoreKey, err := t.Next(&customer)
		if err == datastore.Done {
			break
		}
		if err != nil {
			log.Errorf(c, "%s", "cron.RemoveExpiredCards: Could not retrieve customer data. ", err)
		}

		//expiration in datastore is stored as MM/YYYY or M/YYYY if month is Jan. through Sep.
		//split the expiration into month and year so we can check if the month is in format M or MM
		expSplit := strings.Split(customer.CardExpiration, "/")
		if len(expSplit[0]) == 1 {
			customer.CardExpiration = "0" + customer.CardExpiration
		}

		//parse expiration into a time.Time
		expiration, err := time.Parse("01/2006", customer.CardExpiration)
		if err != nil {
			log.Errorf(c, "%s", "cron.RemoveExpiredCards: Could not parse expiration into a time.Time. ", customer.CardExpiration, err)
		}

		//check if expiration is in the past
		if expiration.Sub(now) < 0 {
			//card is expired
			//remove the card from the datastore, from stripe, and refresh memcache
			log.Infof(c, "%s", "cron.RemoveExpiredCards: Card is expired. ", customer.CustomerName, customer.CardExpiration)

			//get datastore id as a string
			datastoreID := strconv.FormatInt(datastoreKey.IntID(), 10)

			err := card.RemoveDo(datastoreID, r)
			if err != nil {
				log.Errorf(c, "%v", "cron.RemoveExpiredCards: Could not remove card.", customer.CustomerName, err)
			}
		}
	}

	log.Infof(c, "%s", "cron.RemoveExpiredCards: Time elapsed: ", time.Since(now))
	fmt.Fprint(w, "done")
	return
}
