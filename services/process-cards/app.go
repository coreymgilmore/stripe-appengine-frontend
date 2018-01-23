/*
Package stripeappenginefrontend implements a simple web app for collecting and charging
credit cards.

This app was designed for use in companies that collect their clients cards and process
charges without the client needing to provide card information with every purchase.  This
is sort of a "virtual terminal".  Companies that use this app don't have to store credit
card information to reduce security issues (or PCI compliance).  The app provides one main
user interface and follows a "single page app" style.

This app was designed to run on Google App Engine and will not work in a normal golang environment.
Data is stored in Google Cloud Datastore.  This is a NoSQL like database.  The only data we really
store is user credentials, basic company information for receipts, and a list of customers who
we will process charges for.  Most of this data is also stored in memcache to reduce the usage of
Cloud Datastore for less latency and less Google Cloud fees.

Payment are processed by either manually entering the payment amount or via an api-style http request.
There is no need to store the credit card's information (number, expiration, security code).
The card's information is saved to Stripe and only an id is saved to the App Engine datastore. This id
is used to process the card with Stripe.
*/
package stripeappenginefrontend

import (
	"net/http"

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

func init() {

	//**********************************************************************
	//Middleware
	a := alice.New(middleware.Auth)
	admin := alice.New(middleware.Auth, middleware.Administrator)
	add := admin.Append(middleware.AddCards)
	remove := admin.Append(middleware.RemoveCards)
	charge := admin.Append(middleware.ChargeCards)
	reports := admin.Append(middleware.ViewReports)

	//**********************************************************************
	//Router
	r := mux.NewRouter()
	r.StrictSlash(true)

	//basic pages
	r.HandleFunc("/", pages.Root)
	r.HandleFunc("/setup/", pages.CreateAdminShow)
	r.HandleFunc("/create-admin/", users.CreateAdmin).Methods("POST")
	r.HandleFunc("/login/", users.Login)
	r.HandleFunc("/logout/", users.Logout)
	r.HandleFunc("/diagnostics/", pages.Diagnostics)

	//cron
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

	//company info
	comp := r.PathPrefix("/company").Subrouter()
	comp.Handle("/get/", a.Then(http.HandlerFunc(company.GetAPI))).Methods("GET")
	comp.Handle("/set/", admin.Then(http.HandlerFunc(company.SaveAPI))).Methods("POST")

	//app settings
	as := r.PathPrefix("/app-settings").Subrouter()
	as.Handle("/get/", a.Then(http.HandlerFunc(appsettings.GetAPI))).Methods("GET")
	as.Handle("/set/", admin.Then(http.HandlerFunc(appsettings.SaveAPI))).Methods("POST")

	//Pages that don't exist
	r.NotFoundHandler = http.HandlerFunc(pages.NotFound)

	//Have the server listen on all router endpoints
	http.Handle("/", r)
}
