package pages

import (
	"net/http"
	"strconv"

	"google.golang.org/appengine"

	"card"
	"receipt"
	"sessionutils"
	"templates"
	"users"
)

//STRUCT USED FOR AUTOMATICALLY FILLING IN DATA IN THE "CHARGE" PANEL IF A USER IS LOGGED IN
//user must already be logged in and session token/data is stored
//this is what powers the api-like autofill of the form data
//this struct is used to fill in the data when building the output template for the /main/ page
type autoloader struct {
	Amount   float64
	Invoice  string
	Po       string
	UserData users.User
	CardData card.CustomerDatastore
	Error    interface{}
}

const (
	SESSION_INIT_ERR_TITLE = "Session Initialization Error"
	ADMIN_INIT_ERR_TITLE   = "Admin. Setup Error"
)

//MAIN ROOT PAGE
//not logged in page
func Root(w http.ResponseWriter, r *http.Request) {
	//check that session store was initialized correctly
	if err := sessionutils.CheckSession(); err != nil {
		notificationPage(w, "panel-danger", SESSION_INIT_ERR_TITLE, err, "btn-default", "/", "Go Back")
		return
	}

	//check that stripe private key and statement desecriptor were read correctly
	if err := card.CheckStripe(); err != nil {
		notificationPage(w, "panel-danger", SESSION_INIT_ERR_TITLE, err, "btn-default", "/", "Go Back")
		return
	}

	//check that receipt info was read correctly
	if err := receipt.Check(); err != nil {
		notificationPage(w, "panel-danger", "Receipt Info Error", err, "btn-default", "/", "Go Back")
		return
	}

	//check if the admin user exists
	//redirect user to create admin if it does not exist
	err := users.DoesAdminExist(r)
	if err == users.ErrAdminDoesNotExist {
		http.Redirect(w, r, "/setup/", http.StatusFound)
		return
	} else if err != nil {
		notificationPage(w, "panel-danger", ADMIN_INIT_ERR_TITLE, err, "btn-default", "/", "Go Back")
		return
	}

	//check if user is already signed in
	//if user is already logged in, redirect to /main/ page
	session := sessionutils.Get(r)
	if session.IsNew == false {
		uId := session.Values["user_id"].(int64)
		c := appengine.NewContext(r)
		u, err := users.Find(c, uId)
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
	templates.Load(w, "root", nil)
	return
}

//PAGES THAT DO NOT EXIST
//404s
func NotFound(w http.ResponseWriter, r *http.Request) {
	notificationPage(w, "panel-danger", "Page Not Found", "This page does not exist. Please try logging in.", "btn-default", "/", "Log In")
	return
}

//MAIN LOGGED IN PAGE
//load the page and show only the nav buttons & panels that a user has access rights to
//templating hides everything else
//also use url to autofil data into charge panel
//if a link to the page has a "customer_id" form value, this will automatically find the customer's card data and show it in the panel
//if "amount", "invoice", and/or "po" form values are given, these will also automatically be filled into the charge panel's form
//if "customer_id" is not given, no autofilling will occur of any fields
//"amount" must be in cents
//card is no automatically charged, user must still click charge button
func Main(w http.ResponseWriter, r *http.Request) {
	//placeholder for sending data back to template
	var tempData autoloader

	//get logged in user data
	//catch instances where session is not working and redirect user to log in page
	session := sessionutils.Get(r)
	if session.IsNew == true {
		notificationPage(w, "panel-danger", "Cannot Load Page", "Your session has expired or there is an error.  Please try logging in again or contact an administrator.", "btn-default", "/", "Log In")
		return
	}
	userId := session.Values["user_id"].(int64)
	c := appengine.NewContext(r)
	user, err := users.Find(c, userId)
	if err != nil {
		notificationPage(w, "panel-danger", "Cannot Load Page", err, "btn-default", "/", "Try Again")
		return
	}

	//check for url form values for autofilling charge panel
	//if data in url does not exist, just load the page with user data only
	custId := r.FormValue("customer_id")
	if len(custId) == 0 {
		tempData.UserData = user
		templates.Load(w, "main", tempData)
		return
	}

	//data in url does exist
	//look up card data by customer id
	//get the card data to show in the panel so user can visually confirm they are charging the correct card
	//if an error occurs, just load the page normally
	custData, err := card.FindByCustId(c, custId)
	if err != nil {
		tempData.Error = "The form could not be autofilled because the customer ID you provided could not be found.  The ID is either incorrect or the customer's credit card has not been added yet."
		tempData.UserData = user
		templates.Load(w, "main", tempData)
		return
	}

	//if amount was given, it is in cents
	//display it in input as dollars
	//check for other form values and build template
	amountUrl := r.FormValue("amount")
	amountFloat, _ := strconv.ParseFloat(amountUrl, 64)
	amountDollars := amountFloat / 100
	tempData.Amount = amountDollars
	tempData.Invoice = r.FormValue("invoice")
	tempData.Po = r.FormValue("po")
	tempData.CardData = custData
	tempData.UserData = user
	templates.Load(w, "main", tempData)
	return
}

//LOAD THE PAGE TO CREATE THE INITIAL ADMIN USER
//this is loaded upon the "first run" of the app and that should be it
//first run is when administrator user does not exist in datastore
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

//HELPER FUNC TO SHOW NOTIFICAITON PAGE
//less retyping
//panelType is "panel-default", "panel-danger", etc.
//title is the text in the panel-heading
//btnType is "ben-default", etc.
//btnPath is the link to the page where the btn redirects
func notificationPage(w http.ResponseWriter, panelType, title string, err interface{}, btnType, btnPath, btnText string) {
	templates.Load(w, "notifications", templates.NotificationPage{panelType, title, err, btnType, btnPath, btnText})
	return
}

//GET DIAGNOSTICS FOR APP
func Diagnostics(w http.ResponseWriter, r * http.Request) {
	c := appengine.NewContext(r)

	out := map[string]interface{}{
		"App ID": appengine.AppID(c),
		"Instance ID": appengine.InstanceID(),
		"Version ID": appengine.VersionID(c),
		"Datacenter": appengine.Datacenter(c),
		"Module Name": appengine.ModuleName(c),
		"Server Software": appengine.ServerSoftware(),
	}

	templates.Load(w, "diagnostics", out)
	return
}