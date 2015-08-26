package middleware

import (
	"net/http"
	"errors"
	
	"appengine"
	
	"sessionutils"
	"users"
	"templates"
	"output"
)

var ErrNotAuthorized = errors.New("userDoesNotHavePermission")

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
			output.Error(err, "An error occured in the middleware.", w)
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

//CHECK ACCESS RIGHTS
func AddCards(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		//get session data
		session := sessionutils.Get(r)

		//look up user data
		c := 			appengine.NewContext(r)
		userId := 		session.Values["user_id"].(int64)
		data, err := 	users.Find(c, userId)
		if err != nil {
			output.Error(err, "An error occured in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.AddCards == false {
			output.Error(ErrNotAuthorized, "You do not have permission to add new cards.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
	})
}

func RemoveCards(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		//get session data
		session := sessionutils.Get(r)

		//look up user data
		c := 			appengine.NewContext(r)
		userId := 		session.Values["user_id"].(int64)
		data, err := 	users.Find(c, userId)
		if err != nil {
			output.Error(err, "An error occured in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.RemoveCards == false {
			output.Error(ErrNotAuthorized, "You do not have permission to remove cards.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
	})
}

func ChargeCards(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		//get session data
		session := sessionutils.Get(r)

		//look up user data
		c := 			appengine.NewContext(r)
		userId := 		session.Values["user_id"].(int64)
		data, err := 	users.Find(c, userId)
		if err != nil {
			output.Error(err, "An error occured in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.ChargeCards == false {
			output.Error(ErrNotAuthorized, "You do not have permission to charge or refund cards.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
	})
}

func ViewReports(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		//get session data
		session := sessionutils.Get(r)

		//look up user data
		c := 			appengine.NewContext(r)
		userId := 		session.Values["user_id"].(int64)
		data, err := 	users.Find(c, userId)
		if err != nil {
			output.Error(err, "An error occured in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.ViewReports == false {
			output.Error(ErrNotAuthorized, "You do not have permission to view reports.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
	})
}

func Administrator(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		//get session data
		session := sessionutils.Get(r)

		//look up user data
		c := 			appengine.NewContext(r)
		userId := 		session.Values["user_id"].(int64)
		data, err := 	users.Find(c, userId)
		if err != nil {
			output.Error(err, "An error occured in the middleware.", w)
			return
		}

		//check if user can add cards
		if data.Administrator == false {
			output.Error(ErrNotAuthorized, "You are not an administrator therefore you cannot access this page.", w)
			return
		}

		//move to next middleware or handler
		next.ServeHTTP(w, r)
	})
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
