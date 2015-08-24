package middleware

import (
	"net/http"
	"fmt"
	
	"appengine"
	
	"sessionutils"
	"users"
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

		//look up user in memcache and/or datastore
		c := 			appengine.NewContext(r)
		userId := 		session.Values["user_id"].(int64)
		data, err := 	users.Find(c, userId)
		if err != nil {
			fmt.Fprint(w, err)
			return
		}

		//check if user is allowed access
		if users.AllowedAccess(data) == false {
			sessionutils.Destroy(w, r)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		//okay
		//extend expirtation of session cookie
		sessionutils.ExtendExpiration(session, w, r)
		
		//move to next middleware or handler
		next.ServeHTTP(w, r)
	})
}
