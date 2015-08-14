package middleware

import (
	"net/http"

	"sessionutils"
)

//MIDDLEWARE TO CHECK IF A USER IS LOGGED IN
//checks if user is logged in on every page
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		//get use data from session
		session := sessionutils.Get(r)
		
		//session data does not exist yet
		//this is a new session
		//redirect user to log in page
		if session.IsNew {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		//get values from session

		//look up user data
		//user username, userId, and token as criteria since all have to match one user

		//does a user with this data exist

		//everything is ok
		//extend expiration of session cookie
		sessionutils.ExtendExpiration(session, w, r)
		
		//move to next middleware or handler
		next.ServeHTTP(w, r)
	})
}