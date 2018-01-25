/*
Package pages implements functions to display the app's interface, the UI.
*/
package pages

import (
	"net/http"
	"strconv"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/appsettings"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/card"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/company"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/templates"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

//autoLoader is used when making api-style semi-automated request to charge a card
//User must be logged in to the app already for this to work, otherwise user is shown a
//login page.
//This data is grabbed from the url and auto filled into the app's interface so all a
//user has to do is click the "charge" button.
type autoloader struct {
	Amount              float64                //the amount we want to charge, in dollars.cents.
	Invoice             string                 //extra info
	Po                  string                 //" "
	UserData            users.User             //data on the user who is processing this charge, retrieved from session data
	CardData            card.CustomerDatastore //data on the customer/card we want to charge
	AppSettings         appsettings.Settings   //data to modify the look of the app
	CompanyInfo         company.Info           //data about the company, used to check if info for receipt and statement descriptor are set
	HasCompanyInfoError bool                   //true if company info or statement descriptor are missing/blank
	Error               interface{}
}

const (
	//text to display when certain errors occur
	//defined as constants in case they need to be changed in the future
	//or reused for other purposes
	sessionInitError = "Session Initialization Error"
	adminInitError   = "Admin Setup Error"
)

//Root is used to show the login page of the app
//When a user browses to this page (usually just the domain), the user is checked for a session.
//If a session exists, the app attempts to auto-login the user.
//Otherwise a user is shown the log in prompt.
//This also handles the "first run" of the app in which no users exist yet. It forces creation of the "super admin".
func Root(w http.ResponseWriter, r *http.Request) {
	//check that session store was initialized correctly
	if err := sessionutils.CheckInit(r); err != nil {
		notificationPage(w, "panel-danger", sessionInitError, err, "btn-default", "/", "Go Back")
		return
	}

	//check that stripe private key was set correctly
	if err := card.CheckInit(); err != nil {
		notificationPage(w, "panel-danger", sessionInitError, err, "btn-default", "/", "Go Back")
		return
	}

	//check if the admin user exists
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
		userID := session.Values["user_id"].(int64)
		c := appengine.NewContext(r)
		u, err := users.Find(c, userID)
		if err != nil {
			sessionutils.Destroy(w, r)
			notificationPage(w, "panel-danger", "Autologin Error", "There was an issue looking up your user account. Please go back and try logging in.", "btn-default", "/", "Go Back")
			return
		}

		//user data was found
		//check if user is allowed access
		if users.AllowedAccess(u) == false {
			sessionutils.Destroy(w, r)
			notificationPage(w, "panel-danger", "Autologin Error", "You are not allowed access. Please contact an administrator.", "btn-default", "/", "Go Back")
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
//404s
func NotFound(w http.ResponseWriter, r *http.Request) {
	notificationPage(w, "panel-danger", "Page Not Found", "This page does not exist. Please try logging in.", "btn-default", "/", "Log In")
	return
}

//Main loads the main UI of the app
//This is the page the user sees once they are logged in.
//This ui is a single page app and holds almost all the functionality of the app.
//The user only sees the parts of the ui they have access to, the rest is removed
//via golang contemplating.
//We also check if this page was loaded with a bunch of extra data in the url, this
//would be used to perform the api-like semi-automated charging of the card.
//If a link to the page has a "customer_id" form value, this will automatically find
//the customer's card data and show it in the panel.
//If "amount", "invoice", and/or "po" form values are given, these will also automatically
//be filled into the charge panel's form.
//If "customer_id" is not given, no auto filling will occur of any fields.
//"amount" must be in cents.
//Card is not automatically charged, user still has to click "charge" button.
func Main(w http.ResponseWriter, r *http.Request) {
	//placeholder for sending data back to template
	var templateData autoloader

	//get logged in user data
	//catch instances where session is not working and redirect user to log in page
	//use the user's data to show/hide certain parts of the ui per the users access rights
	session := sessionutils.Get(r)
	if session.IsNew == true {
		notificationPage(w, "panel-danger", "Cannot Load Page", "Your session has expired or there is an error.  Please try logging in again or contact an administrator.", "btn-default", "/", "Log In")
		return
	}
	userID := session.Values["user_id"].(int64)

	//look up data for this user
	c := appengine.NewContext(r)
	user, err := users.Find(c, userID)
	if err != nil {
		log.Errorf(c, "%+v", "pages.Main: look up user data", err)
		notificationPage(w, "panel-danger", "Cannot Load Page", err, "btn-default", "/", "Try Again")
		return
	}
	templateData.UserData = user

	//look up app settings
	//if app settings don't exist yet (upgrading from an older version of this app),
	//this will return default values
	//we do this so the app will still work even when app settings haven't been set yet.
	as, err := appsettings.Get(r)
	if err != nil {
		log.Errorf(c, "%+v", "pages.Main: look up app settings", err)
		notificationPage(w, "panel-danger", "Cannot Load Page", err, "btn-default", "/", "Try Again")
		return
	}
	templateData.AppSettings = as

	//look up company data so we can check if statement descriptor is set
	//we need the statement descriptor for charging cards
	//this used to be in app.yaml environmental variable but is now in datastore
	//this catches instances where this app was upgraded but the statement descriptor hasn't been set yet.
	compData, err := company.Get(r)
	if err != nil {
		log.Errorf(c, "%+v", "pages.Main: look up company info", err)
		notificationPage(w, "panel-danger", "Cannot Load Page", err, "btn-default", "/", "Try Again")
		return
	}
	templateData.CompanyInfo = compData

	//check if company data exists (isn't blank/default)
	if compData.CompanyName == "" || compData.StatementDescriptor == "" {
		templateData.HasCompanyInfoError = true
	}

	//check for url form values for autofilling charge panel
	//if data in url does not exist, just load the page with user data only
	custID := r.FormValue("customer_id")
	if len(custID) == 0 {
		templates.Load(w, "main", templateData)
		return
	}

	//data in url does exist
	//look up card data by customer id
	//get the card data to show in the panel so user can visually confirm they are charging the correct card
	//if an error occurs, just load the page normally
	custData, err := card.FindByCustomerID(c, custID)
	if err != nil {
		templateData.Error = "The form could not be autofilled because the customer ID you provided could not be found.  The ID is either incorrect or the customer's credit card has not been added yet."
		templates.Load(w, "main", templateData)
		return
	}
	templateData.CardData = custData

	//if amount was given, it is in cents
	//display it in html input as dollars
	amountURL := r.FormValue("amount")
	amountFloat, _ := strconv.ParseFloat(amountURL, 64)
	amountDollars := amountFloat / 100
	templateData.Amount = amountDollars

	//check for other form values and build template
	templateData.Invoice = r.FormValue("invoice")
	templateData.Po = r.FormValue("po")

	//load the page with the card data
	templates.Load(w, "main", templateData)
	return
}

//CreateAdminShow loads the page used to create the initial admin user
//this is done only upon the app running for the first time (per project on app engine since nothing exists in this project's datastore yet)
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
//less retyping
//panelType is "panel-default", "panel-danger", etc.
//title is the text in the panel-heading
//btnType is "ben-default", etc.
//btnPath is the link to the page where the btn redirects
func notificationPage(w http.ResponseWriter, panelType, title string, err interface{}, btnType, btnPath, btnText string) {
	templates.Load(w, "notifications", templates.NotificationPage{panelType, title, err, btnType, btnPath, btnText})
	return
}

//Diagnostics shows a bunch of app engine's information for the app/project
//useful for figuring out which version of an app is serving
func Diagnostics(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	out := map[string]interface{}{
		"App ID":                   appengine.AppID(c),
		"Instance ID":              appengine.InstanceID(),
		"Default Version Hostname": appengine.DefaultVersionHostname(c),
		"Version ID":               appengine.VersionID(c),
		"Datacenter":               appengine.Datacenter(c),
		"Module Name":              appengine.ModuleName(c),
		"Server Software":          appengine.ServerSoftware(),
	}

	templates.Load(w, "diagnostics", out)
	return
}
