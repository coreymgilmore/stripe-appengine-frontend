/*
Package middleware handles authentication and access right to the app.

User is checked on every endpoint or page load to make sure the user's password has not changed, the user's account
is active, and if the user's session is still active. This then extends a user's session if the user is valid so
that the user is "auto logged in" to the app upon loading it.

Access rights determine what elements of the GUI the user can see and interact with as well as limits
usage of certain endpoints.
*/
package middleware

import (
	"errors"
	"log"
	"net/http"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/templates"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
)

//errNotAuthorized is returned when user does not have access rights to certain functionality
var errNotAuthorized = errors.New("middleware: user does not have permission")

//Auth checks if a user is logged in and is allowed access to the app
//this is done on every page load and every endpoint
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := r.Context()

		//get user data from session
		session := sessionutils.Get(r)

		//session data does not exist yet
		//this is a new session
		//redirect user to log in page
		if session.IsNew {
			log.Println("middleware.Auth", "Session data does not exist yet.")
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		//check if user id is in session
		//otherwise show user a notice and force user to log in again
		userID := sessionutils.GetUserID(r)
		if userID < 1 {
			sessionutils.Destroy(w, r)
			notificationPage(w, "panel-danger", "Session Expired", "Your session has expired. Please log back in or contact an administrator if this problem persists.", "btn-default", "/", "Log In")
			return
		}

		//look up user in datastore
		data, err := users.Find(c, userID)
		if err != nil {
			sessionutils.Destroy(w, r)
			notificationPage(w, "panel-danger", "Application Error", "The app encountered an error in the middleware while trying to authenticate you as a legitimate user. Please try logging in again or contact an administrator.", "btn-default", "/", "Log In")
			return
		}

		//check if user is allowed access to the app
		//this is a setting the app's administrators can toggle for each user
		if data.Active == false {
			sessionutils.Destroy(w, r)
			log.Println("middleware.Auth", "User not allowed access.")
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		//user is allowed access
		//extend expiration of session cookie to allow user to stay "logged in"
		sessionutils.ExtendExpiration(session, w, r)

		//move to next middleware or handler
		next.ServeHTTP(w, r)
		return
	})
}

//AddCards checks if the user is allowed to add credit cards to the app
func AddCards(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//look up user data
		c := r.Context()
		userID := sessionutils.GetUserID(r)
		data, err := users.Find(c, userID)
		if err != nil {
			log.Println("middleware.AddCards: ", err)
			output.Error(err, "An error occurred in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.AddCards == false {
			output.Error(errNotAuthorized, "You do not have permission to add new cards.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
		return
	})
}

//RemoveCards checks if the user is allowed to remove credit cards from the app
func RemoveCards(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//look up user data
		c := r.Context()
		userID := sessionutils.GetUserID(r)
		data, err := users.Find(c, userID)
		if err != nil {
			log.Println("middleware.RemoveCards: ", err)
			output.Error(err, "An error occurred in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.RemoveCards == false {
			output.Error(errNotAuthorized, "You do not have permission to remove cards.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
		return
	})
}

//ChargeCards checks if the user is allowed to charge credit cards
func ChargeCards(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//look up user data
		c := r.Context()
		userID := sessionutils.GetUserID(r)
		data, err := users.Find(c, userID)
		if err != nil {
			log.Println("middleware.ChargeCards: ", err)
			output.Error(err, "An error occurred in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.ChargeCards == false {
			output.Error(errNotAuthorized, "You do not have permission to charge or refund cards.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
		return
	})
}

//ViewReports checks if the user is allowed to view the charge & refunds reports
func ViewReports(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//look up user data
		c := r.Context()
		userID := sessionutils.GetUserID(r)
		data, err := users.Find(c, userID)
		if err != nil {
			log.Println("middleware.ViewReports: ", err)
			output.Error(err, "An error occurred in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.ViewReports == false {
			output.Error(errNotAuthorized, "You do not have permission to view reports.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
		return
	})
}

//Administrator checks if the user is an administrator to the app
//this allows for adding/removing/changing other users
//also allows for changing the data that shows up on the receipt
func Administrator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//look up user data
		c := r.Context()
		userID := sessionutils.GetUserID(r)
		data, err := users.Find(c, userID)
		if err != nil {
			log.Println("middleware.Administrator: ", err)
			output.Error(err, "An error occurred in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.Administrator == false {
			output.Error(errNotAuthorized, "You are not an administrator therefore you cannot access this page.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
		return
	})
}

//notificationPage is a helper function to load an html template when an error occurs during authentication
//less retyping
//panelType is "panel-default", "panel-danger", etc.
//title is the text in the panel-heading
//btnType is "ben-default", etc.
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
