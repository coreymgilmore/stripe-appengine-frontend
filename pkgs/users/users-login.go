package users

import (
	"log"
	"net/http"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/pwds"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
)

//Login verifies a username and password combo
//This makes sure the user exists, that the password is correct, and that the user is active.
//If user is allowed access, their data is saved to the session and they are redirected into the app.
func Login(w http.ResponseWriter, r *http.Request) {
	//get inputs
	username := r.FormValue("username")
	password := r.FormValue("password")

	//get user data
	c := r.Context()
	id, data, err := getDataByUsername(c, username)
	if err == ErrUserDoesNotExist {
		notificationPage(w, "panel-danger", "Cannot Log In", "The username you provided does not exist.", "btn-default", "/", "Try Again")
		return
	}

	//is user allowed access
	if !data.Active {
		notificationPage(w, "panel-danger", "Cannot Log In", "Your user account is inactive. Please contact an administrator.", "btn-default", "/", "Go Back")
		return
	}

	//validate password
	_, err = pwds.Verify(password, data.Password)
	if err != nil {
		notificationPage(w, "panel-danger", "Cannot Log In", "The password you provided is invalid.", "btn-default", "/", "Try Again")
		return
	}

	//user validated
	//save session data
	session := sessionutils.Get(r)
	if !session.IsNew {
		sessionutils.Destroy(w, r)
		session = sessionutils.Get(r)
	}
	sessionutils.AddValue(session, "username", username)
	sessionutils.AddValue(session, "user_id", id)
	sessionutils.Save(session, w, r)

	//show user main page
	http.Redirect(w, r, "/main/", http.StatusFound)
}

//Logout handles logging out of the app
//this removes the session data so a user must log back in before using the app
func Logout(w http.ResponseWriter, r *http.Request) {
	//destroy session
	sessionutils.Destroy(w, r)

	log.Println("Performing logout via users.Logout.")

	//redirect to root page
	http.Redirect(w, r, "/?ref=logout", http.StatusFound)
}
