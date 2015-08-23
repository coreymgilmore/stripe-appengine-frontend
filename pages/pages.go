package pages 

import (
	"net/http"
	"strconv"
	
	"appengine"

	"templates"
	"sessionutils"
	"card"
	"users"
)

type autoloader struct {
	Amount 			float64 
	Invoice 		string
	Po 				string
	UserData 		users.User
	CardData 		card.CustomerDatastore
	Error 			string
}

//MAIN ROOT PAGE
//not logged in page
func Root(w http.ResponseWriter, r *http.Request) {
	//check that session store was initialized correctly
	if err := sessionutils.CheckSession(); err != nil {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Session Init Error", err, "btn-default", "/", "Try Again"})
		return
	}

	//check that stripe private key and statement desecriptor were read correctly
	if err := card.CheckStripe(); err != nil {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Stripe Init Error", err, "btn-default", "/", "Try Again"})
		return
	}

	//check if the admin user exists
	//redirect user to create admin if it does not exist
	err := users.DoesAdminExist(r)
	if err == users.ErrAdminDoesNotExist {
		http.Redirect(w, r, "/setup/", http.StatusFound)
		return
	} else if err != nil {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Admin Setup Error", err, "btn-default", "/", "Try Again"})
		return
	}

	//load the login page
	templates.Load(w, "root", nil)
	return
}

//PAGES THAT DO NOT EXIST
func NotFound(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("This page cannot be found."))
	return
}

//MAIN LOGGED IN PAGE
//load the page and only show buttons/panels a user can access
//also used as an api endpoint: a link to this page with a "customer id" (provided when creating a customer) will automatically load the customer into the charge panel
//url can have "customer_id" and then other fields such as "amount" (cents), "invoice", and "po"
//card is not charged automatically but data is autofilled
func Main(w http.ResponseWriter, r *http.Request) {
	//placeholder for sending data back to template
	var tempData autoloader

	//get logged in user data
	session := 		sessionutils.Get(r)
	userId := 		session.Values["user_id"].(int64)
	c := 			appengine.NewContext(r)
	user, err := 	users.Find(c, userId)
	if err != nil {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Cannot Load Page", err, "btn-default", "/", "Try Again"})
		return;
	}
	
	//check for url form values for api style page loading
	//if data in url does not exist, just load the page with user data only
	custId := r.FormValue("customer_id")
	if len(custId) == 0 {
		tempData.UserData = user
		templates.Load(w, "main", tempData)
		return
	}

	//data in url does exist
	//look up card data by customer id
	//if an error occurs, just load the page normally
	custData, err := card.FindByCustId(c, custId)
	if err != nil {
		tempData.Error = "The form could not be autofilled because the customer ID you provided could not be found."
		tempData.UserData = user
		templates.Load(w, "main", tempData)
		return
	}

	//if amount is given, is was given in cents
	//display it in input as dollars.cents
	amountUrl := 		r.FormValue("amount")
	amountFloat, _ := 	strconv.ParseFloat(amountUrl, 64)
	amountDollars := 	amountFloat / 100
	tempData.Amount = 	amountDollars

	//card data was found
	//check for other form values and build template
	tempData.Invoice = 		r.FormValue("invoice")
	tempData.Po = 			r.FormValue("po")
	tempData.CardData = 	custData
	tempData.UserData = 	user
	templates.Load(w, "main", tempData)
	return	
}

//LOAD THE PAGE TO CREATE THE INITIAL ADMIN USER
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
