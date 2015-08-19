package pages 

import (
	"net/http"
	"fmt"
	"templates"
	"sessionutils"
	"card"
	"users"
	"appengine"
)

//MAIN ROOT PAGES
func Root(w http.ResponseWriter, r *http.Request) {
	//check that session store was initialized correctly
	if err := sessionutils.CheckSession(); err != nil {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Session Init Error", err, "btn-default", "/", "Try Again"})
		return
	}

	//check if stripe private key is read correctly
	if err := card.CheckStripe(); err != nil {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Stripe Key Error", err, "btn-default", "/", "Try Again"})
		return
	}

	//check if the admin user exists
	//redirect user to create admin if it does not exist
	err := users.DoesAdminExist(r)
	if err == users.ErrAdminDoesNotExist {
		http.Redirect(w, r, "/setup/", http.StatusFound)
		return
	} else if err != nil {
		fmt.Fprint(w, "An error occured while checking if the admin user exists.\n")
		fmt.Fprint(w, err)
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
func Main(w http.ResponseWriter, r *http.Request) {
	//get logged in user data
	session := 		sessionutils.Get(r)
	userId := 		session.Values["user_id"].(int64)
	c := 			appengine.NewContext(r)
	user, err := 	users.Find(c, userId)
	if err != nil {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Cannot Load Page", err, "btn-default", "/", "Try Again"})
		return;
	}

	//display template using stuct to display certain html elements
	templates.Load(w, "main", user)
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
