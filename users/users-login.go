/*
	This is part of the users package.
	This deals with logging a user in and out of the app.
*/

package users

import (
	"net/http"

	"google.golang.org/appengine"

	"github.com/coreymgilmore/pwds"

	"sessionutils"
)

//LOGIN
//verify a username and password combo is correct
//make sure a user account is active
func Login(w http.ResponseWriter, r *http.Request) {
	//get form values
	username := r.FormValue("username")
	password := r.FormValue("password")

	//get user data
	c := appengine.NewContext(r)
	id, data, err := exists(c, username)
	if err == ErrUserDoesNotExist {
		notificationPage(w, "panel-danger", "Cannot Log In", "The username you provided does not exist.", "btn-default", "/", "Try Again")
		return
	}

	//is user allowed access
	if AllowedAccess(data) == false {
		notificationPage(w, "panel-danger", "Cannot Log In", "You are not allowed access. Please contact an administrator.", "btn-default", "/", "Go Back")
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
	if session.IsNew == false {
		sessionutils.Destroy(w, r)
		session = sessionutils.Get(r)
	}
	sessionutils.AddValue(session, "username", username)
	sessionutils.AddValue(session, "user_id", id)
	sessionutils.Save(session, w, r)

	//show user main page
	http.Redirect(w, r, "/main/", http.StatusFound)
	return
}

//LOGOUT
func Logout(w http.ResponseWriter, r *http.Request) {
	//destroy session
	sessionutils.Destroy(w, r)

	//redirect to root page
	http.Redirect(w, r, "/", http.StatusFound)
	return
}
