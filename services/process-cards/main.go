/*
Package main implements a simple web app for collecting and charging credit cards.

This app was designed for use in companies that collect their clients cards and process
charges without the client needing to provide card information with every purchase.
Companies that use this app don't have to store credit card information, or ask for it with
every purchase, to reduce security issues or remove need for PCI compliance.

This app was designed to run on Google App Engine and will not work in a normal golang
environment.  You must have a Google Cloud account.

This app uses Stripe as the payment processor.  You must have a Stripe account set up.

The only data stored for this app is user credentials, basic company information for
receipts, and a list of customers who we will process charges for.  Most of this data is
also cached to reduce the usage of Cloud Datastore for less latency and less Google Cloud
fees.  There is no need to store the credit card's information (number, expiration, security
code).  The card's information is saved to Stripe and only an id is saved to this app. This
id is used to process the card with Stripe.

Payment are processed by either manually choosing the customer and entering the payment amount
or via an api-style http request.
*/
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/appsettings"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/card"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/company"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/cron"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/middleware"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/pages"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/receipt"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
)

//defaultPort is the port used when serving in the dev environment
const defaultPort = "8005"

func init() {
	err := datastoreutils.Connect()
	if err != nil {
		log.Fatalln("Could not connect to datastore.", err)
		return
	}
}

func main() {
	//middleware
	a := alice.New(middleware.Auth)
	admin := a.Append(middleware.Administrator)
	add := a.Append(middleware.AddCards)
	remove := a.Append(middleware.RemoveCards)
	charge := a.Append(middleware.ChargeCards)
	reports := a.Append(middleware.ViewReports)

	//router
	r := mux.NewRouter()
	r.StrictSlash(true)

	//basic pages
	r.HandleFunc("/", pages.Root)
	r.HandleFunc("/setup/", pages.CreateAdminShow)
	r.HandleFunc("/create-admin/", users.CreateAdmin).Methods("POST")
	r.HandleFunc("/login/", users.Login)
	r.HandleFunc("/logout/", users.Logout)
	r.HandleFunc("/diagnostics/", pages.Diagnostics)

	//cron tasks
	r.HandleFunc("/cron/remove-expired-cards/", cron.RemoveExpiredCards)

	//main app page once user is logged in
	main := http.HandlerFunc(pages.Main)
	r.Handle("/main/", a.Then(main))

	//API endpoints
	//users
	u := r.PathPrefix("/users").Subrouter()
	u.Handle("/add/", admin.Then(http.HandlerFunc(users.Add))).Methods("POST")
	u.Handle("/get/", a.Then(http.HandlerFunc(users.GetOne))).Methods("GET")
	u.Handle("/get/all/", admin.Then(http.HandlerFunc(users.GetAll))).Methods("GET")
	u.Handle("/change-pwd/", admin.Then(http.HandlerFunc(users.ChangePwd))).Methods("POST")
	u.Handle("/update/", admin.Then(http.HandlerFunc(users.UpdatePermissions))).Methods("POST")

	//cards
	c := r.PathPrefix("/card").Subrouter()
	c.Handle("/add/", add.Then(http.HandlerFunc(card.Add))).Methods("POST")
	c.Handle("/get/", a.Then(http.HandlerFunc(card.GetOne))).Methods("GET")
	c.Handle("/get/all/", a.Then(http.HandlerFunc(card.GetAll))).Methods("GET")
	c.Handle("/remove/", remove.Then(http.HandlerFunc(card.RemoveAPI))).Methods("POST")
	c.Handle("/charge/", charge.Then(http.HandlerFunc(card.Charge))).Methods("POST")
	c.Handle("/receipt/", a.Then(http.HandlerFunc(receipt.Show))).Methods("GET")
	c.Handle("/report/", reports.Then(http.HandlerFunc(card.Report))).Methods("GET")
	c.Handle("/refund/", charge.Then(http.HandlerFunc(card.Refund))).Methods("POST")

	c.Handle("/auto-charge/", http.HandlerFunc(card.AutoCharge)).Methods("POST")

	//company info
	comp := r.PathPrefix("/company").Subrouter()
	comp.Handle("/get/", a.Then(http.HandlerFunc(company.GetAPI))).Methods("GET")
	comp.Handle("/set/", admin.Then(http.HandlerFunc(company.SaveAPI))).Methods("POST")
	comp.Handle("/preview-receipt/", admin.Then(http.HandlerFunc(receipt.Preview))).Methods("GET")

	//app settings
	as := r.PathPrefix("/app-settings").Subrouter()
	as.Handle("/get/", a.Then(http.HandlerFunc(appsettings.GetAPI))).Methods("GET")
	as.Handle("/set/", admin.Then(http.HandlerFunc(appsettings.SaveAPI))).Methods("POST")
	as.Handle("/generate-api-key", admin.Then(http.HandlerFunc(appsettings.GenerateAPIKey))).Methods("GET")

	//Pages that don't exist
	r.NotFoundHandler = http.HandlerFunc(pages.NotFound)

	//Have the server listen
	log.Println("Starting stripe-appengine-frontend...")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8005"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), r))
}
