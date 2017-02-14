package cron

import (
	"card"
	"fmt"
	"net/http"

	"google.golang.org/appengine"
)

func RemoveExpiredCards(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "running cron")

	c := appengine.NewContext(r)
	card.DatastoreFindMany(c)

	fmt.Fprint(w, "...done")
	return
}
