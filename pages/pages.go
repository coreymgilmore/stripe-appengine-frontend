package pages 

import (
	"net/http"
	"fmt"

	"templates"
	"sessionutils"
	"card"
	"users"
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
	w.Write([]byte("This page does not exist and cannot be found. :("))
	return
}

//MAIN LOGGED IN PAGE
func Main(w http.ResponseWriter, r *http.Request) {
	templates.Load(w, "main", nil)
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
