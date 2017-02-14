/*
This is the main entry point for the app.
This app was designed to run on Google App Engine and will not work in a normal go environment.

The app provide one main user interface and follows a "single page webapp" style.
All actions on the app perform api requests via ajax to other endpoints to perform the task.

This app provide a "virtual terminal" of sorts to save customer credit cards and process payment for orders.
It allows users to add and remove cards just using the company name (the company a card is used for).
Payment are processed by either manually entering the payment amount or via an api-style http request.
There is no need to store the credit card's information (number, expiration, security code).
The card's information is saved to Stripe and only an id is saved to the App Engine datastore. This id
is used to process the card with Stripe.
*/

package stripeappenginefrontend

import (
	"card"
	"cron"
	"middleware"
	"net/http"
	"pages"
	"receipt"
	"users"

	"github.com/gorilla/mux"
	"github.com/justinas/alice"
)

func init() {

	//**********************************************************************
	//Middleware
	//for handling routing a bit better
	a := alice.New(middleware.Auth)
	admin := alice.New(middleware.Auth, middleware.Administrator)
	add := alice.New(middleware.Auth, middleware.AddCards)
	remove := alice.New(middleware.Auth, middleware.RemoveCards)
	charge := alice.New(middleware.Auth, middleware.ChargeCards)
	reports := alice.New(middleware.Auth, middleware.ViewReports)

	//**********************************************************************
	//Router
	r := mux.NewRouter()
	r.StrictSlash(true)

	//root & setup pages
	r.HandleFunc("/", pages.Root)
	r.HandleFunc("/setup/", pages.CreateAdminShow)
	r.HandleFunc("/create-admin/", users.CreateAdmin).Methods("POST")
	r.HandleFunc("/login/", users.Login)
	r.HandleFunc("/logout/", users.Logout)

	//diagnostics page
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
	c.Handle("/remove/", remove.Then(http.HandlerFunc(card.Remove))).Methods("POST")
	c.Handle("/charge/", charge.Then(http.HandlerFunc(card.Charge))).Methods("POST")
	c.Handle("/receipt/", a.Then(http.HandlerFunc(receipt.Show))).Methods("GET")
	c.Handle("/report/", reports.Then(http.HandlerFunc(card.Report))).Methods("GET")
	c.Handle("/refund/", charge.Then(http.HandlerFunc(card.Refund))).Methods("POST")

	//company info
	comp := r.PathPrefix("/company").Subrouter()
	comp.Handle("/get/", a.Then(http.HandlerFunc(receipt.GetCompanyInfo))).Methods("GET")
	comp.Handle("/set/", admin.Then(http.HandlerFunc(receipt.SaveCompanyInfo))).Methods("POST")

	//Pages that don't exist
	r.NotFoundHandler = http.HandlerFunc(pages.NotFound)

	//Have the server listen on all router endpoints
	http.Handle("/", r)
}
