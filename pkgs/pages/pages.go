/*
Package pages implements functions to display the app's interface, the UI.
*/
package pages

import (
	"log"
	"net/http"
	"strconv"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/appsettings"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/card"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/company"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/templates"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
)

//templateData is any data we use to build templates
//this data is injected into the templates when build or used to build the gui
type templateData struct {
	UserData             users.User           //data on the logged in user, used to show/hide certain functionality
	AppSettings          appsettings.Settings //data to modify the look of the app
	CompanyInfo          company.Info         //data about the company, used to check if contact info and statement descriptor are set
	HasCompanyInfoError  bool                 //true if company info or statement descriptor are missing/blank
	StripePublishableKey string               //the stripe key used by stripe.js
	AutofillCard         autofillCardData     //used for autofilling the charge card form
	Error                interface{}          //any error messages
}

//autofillCardData is the data we need to autofill the charge card form
type autofillCardData struct {
	CardData card.CustomerDatastore //data on the customer/card we want to charge
	Amount   float64                //the amount to charge in dollars
	Invoice  string                 //invoice number
	Po       string                 //purchase order number
}

//text to display when certain errors occur
const (
	sessionInitError = "Session Initialization Error"
	adminInitError   = "Initial Setup Error"
)

//Login is used to show the login page of the app
//User is shown a login prompt unless they are already logged in (via data in session).
//If user is already logged in the user is redirected to the main page of the app.
//This also checks if the app is being run for the first time in which no users exist.
//This then instead redirects to the initial setup page.
func Login(w http.ResponseWriter, r *http.Request) {
	//check if the initial user/admin user exists
	//redirect user to create admin if it does not exist
	err := users.DoesAdminExist(r)
	if err == users.ErrAdminDoesNotExist {
		http.Redirect(w, r, "/setup/", http.StatusFound)
		return
	} else if err != nil {
		notificationPage(w, "panel-danger", adminInitError, err, "btn-default", "/", "Go Back")
		return
	}

	//check if user is already signed in
	//if user is already logged in, redirect to /main/ page
	session := sessionutils.Get(r)
	if session.IsNew == false {
		userID := sessionutils.GetUserID(r)
		c := r.Context()
		u, err := users.Find(c, userID)
		if err != nil {
			sessionutils.Destroy(w, r)
			notificationPage(w, "panel-danger", "Login Error", "There was an issue looking up your user account. Please go back and try logging in.", "btn-default", "/", "Go Back")
			return
		}

		//user data was found
		//check if user is allowed access
		if u.Active == false {
			sessionutils.Destroy(w, r)
			notificationPage(w, "panel-danger", "Login Error", "You are not allowed access. Please contact an administrator.", "btn-default", "/", "Go Back")
		}

		//user account is found an allowed access
		//redirect user
		http.Redirect(w, r, "/main/", http.StatusFound)
		return
	}

	//load the login page
	templates.Load(w, "login", nil)
	return
}

//NotFound is run when a user browses to a pages that does not exists
func NotFound(w http.ResponseWriter, r *http.Request) {
	notificationPage(w, "panel-danger", "Page Not Found", "This page does not exist. Please try logging in.", "btn-default", "/", "Log In")
	return
}

//Main loads the main UI of the app
//This is the page the user sees once they are logged in and the page most actions are performed on.
//This page can be linked to with a bunch of extra data in teh url to autofill the charge card form.
//  If a link to the page has a "customer_id" form value, this will automatically find
//  the customer's card data and show it in the panel.
//  If "amount", "invoice", and/or "po" form values are given, these will also automatically
//  be filled into the charge panel's form.
//  If "customer_id" is not given, no auto filling will occur of any fields.
//  "amount" must be in cents.
//  Card is not automatically charged, user still has to click "charge" button.
func Main(w http.ResponseWriter, r *http.Request) {
	//placeholder for sending data back to template
	var templateData templateData

	//get logged in user data
	//catch instances where session is not working and redirect user to log in page
	//use the user's data to show/hide certain parts of the ui per the users access rights
	session := sessionutils.Get(r)
	if session.IsNew == true {
		notificationPage(w, "panel-danger", "Cannot Load Page", "Your session has expired or there is an error.  Please try logging in again or contact an administrator.", "btn-default", "/", "Log In")
		return
	}
	userID := sessionutils.GetUserID(r)

	//look up data for this user
	c := r.Context()
	user, err := users.Find(c, userID)
	if err != nil {
		log.Println("pages.Main: look up user data", err)
		notificationPage(w, "panel-danger", "Cannot Load Page", err, "btn-default", "/", "Try Again")
		return
	}
	templateData.UserData = user

	//look up app settings
	//if app settings don't exist yet (upgrading from an older version of this app) this will return default values
	//we do this so the app will still work even when app settings haven't been set yet.
	as, err := appsettings.Get(r)
	if err != nil {
		log.Println("pages.Main: look up app settings", err)
		notificationPage(w, "panel-danger", "Cannot Load Page", err, "btn-default", "/", "Try Again")
		return
	}
	templateData.AppSettings = as

	//look up company data so we can check if statement descriptor is set
	//we need the statement descriptor for charging cards
	//this catches instances where this app was upgraded but the statement descriptor hasn't been set yet.
	compData, err := company.Get(r)
	if err != nil {
		log.Println("pages.Main: look up company info", err)
		notificationPage(w, "panel-danger", "Cannot Load Page", err, "btn-default", "/", "Try Again")
		return
	}
	templateData.CompanyInfo = compData

	//check if company data exists (isn't blank/default)
	if compData.CompanyName == "" || compData.StatementDescriptor == "" {
		templateData.HasCompanyInfoError = true
	}

	//save the stripe publishable key to the template data
	//so we can set this in the ui so we are able to create customers
	//stripe key is injected into template in this manner so user deploying
	//this app doesn't have to deal with any <scrip> tags or .js files
	templateData.StripePublishableKey = card.Config.StripePublishableKey

	//check for url form values for autofilling charge panel
	custID := r.FormValue("customer_id")

	//data in url does exist
	//look up card data by customer id
	//get the card data to show in the panel so user can visually confirm they are charging the correct card
	//if an error occurs, just load the page normally
	if len(custID) > 1 {
		autofillData := autofillCardData{}
		custData, err := card.FindByCustomerID(c, custID)
		if err != nil {
			templateData.Error = "The form could not be autofilled because the customer ID you provided could not be found.  The ID is either incorrect or the customer's credit card has not been added yet."
			templates.Load(w, "main", templateData)
			return
		}
		autofillData.CardData = custData

		//if amount was given, it is in cents
		//display it in html input as dollars
		amountURL := r.FormValue("amount")
		amountFloat, _ := strconv.ParseFloat(amountURL, 64)
		amountDollars := amountFloat / 100
		autofillData.Amount = amountDollars

		//check for other form values and build template
		autofillData.Invoice = r.FormValue("invoice")
		autofillData.Po = r.FormValue("po")

		//save the autofill data to the template data
		templateData.AutofillCard = autofillData
	}

	//load the page
	templates.Load(w, "main", templateData)
	return
}

//CreateAdminShow loads the page used to create the initial admin user
//this is done only upon the app running for the first time (since nothing exists in this project's datastore yet)
func CreateAdminShow(w http.ResponseWriter, r *http.Request) {
	//check if the admin user already exists
	//no need to show this page if it does exist
	err := users.DoesAdminExist(r)
	if err == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	templates.Load(w, "create-admin", nil)
	return
}

//notificationPage is a helper func to show notification page
//panelType is "panel-default", "panel-danger", etc.
//title is the text in the panel-heading
//btnType is "btn-default", etc.
//btnPath is the link to the page where the btn redirects
func notificationPage(w http.ResponseWriter, panelType, title string, err interface{}, btnType, btnPath, btnText string) {
	data := templates.NotificationPage{
		PanelColor: panelType,
		Title:      title,
		Message:    err,
		BtnColor:   btnType,
		LinkHref:   btnPath,
		BtnText:    btnText,
	}

	templates.Load(w, "notifications", data)
	return
}
