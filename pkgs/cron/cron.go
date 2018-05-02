package cron

import (
	"net/http"
	"strconv"
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/card"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

//RemoveExpiredCards removes old cards
//This works by removing any card whose expiration is in in the prior past month.
//This is designed to run monthly as a cron task.
func RemoveExpiredCards(w http.ResponseWriter, r *http.Request) {
	//get context
	c := appengine.NewContext(r)

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

	log.Infof(c, "%s", "Removing expired cards for: ", monthYear)

	//query datastore
	//need customer name for logging and stripe token to remove card from stripe
	fields := []string{"CustomerName", "StripeCustomerToken"}
	q := datastore.NewQuery("card").Filter("CardExpiration =", monthYear).Project(fields...)

	//iterate through results
	//only results should be cards that expired last month
	cardsRemovedCount := 0
	for t := q.Run(c); ; {
		var customer card.CustomerDatastore
		datastoreKey, err := t.Next(&customer)
		if err == datastore.Done {
			break
		}
		if err != nil {
			log.Errorf(c, "%s", "cron.RemoveExpiredCards: Could not retrieve customer data. ", err)
			return
		}

		//remove the card from the datastore, from stripe, and refresh memcache
		datastoreID := strconv.FormatInt(datastoreKey.IntID(), 10)
		_ = datastoreID
		err = card.Remove(datastoreID, r)
		if err != nil {
			log.Errorf(c, "%v", "cron.RemoveExpiredCards: Could not remove card.", customer.CustomerName, err)
			return
		}

		cardsRemovedCount++
	}

	log.Infof(c, "%s", "Removed cards:", cardsRemovedCount)
	log.Infof(c, "%s", "Removing expired cards...done")
	return
}
