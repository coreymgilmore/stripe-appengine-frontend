/*
Package sessionutils implements functionality to more easily deal with sessions for users.
This wraps around gorilla/sessions to make code clearer and more usable.

Session data stores user authentication data in a cookie.
The cookie value is encrypted and authenticated via gorilla/sessions.
*/
package sessionutils

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/sessions"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

//default cookie info
const (
	sessionCookieName   = "session_id"
	sessionCookieDomain = "."
)

//keys are a fixed and required size
//this is the strongest settings as defined by gorilla/sessions
const (
	authKeyLength    = 64
	encryptKeyLength = 32
)

//store is a variable for dealing with session data
var store *sessions.CookieStore

//options for sessions
var options = &sessions.Options{
	Domain: sessionCookieDomain,
	Path:   "/",

	//cookie for session expires in 7 days
	MaxAge: 60 * 60 * 24 * 7,

	//this stops client side scripts from accessing the cookie
	//should be set to true in production
	HttpOnly: true,

	//this will only send/set the session cookie if the website is being served over https
	//should be set to true in production (since stripe requires this website be served over https to begin with)
	//should be set to false when using the appengine dev server
	Secure: false,
}

//init func errors
//since init() cannot return errors, we check for errors upon the app starting up
var (
	initError             error
	initErrorVal          interface{}
	ErrAuthKeyWrongSize   = errors.New("Session: Auth key is invalid. Provide an auth key in app.yaml that is exactly 64 bytes long.")
	ErrEncyptKeyWrongSize = errors.New("Session: Encrypt key is invalid. Provide an encrypt key in app.yaml that is exactly 32 bytes long.")
)

//init initializes the session store
//this reads and sets the auth and encryption keys for session cookie
func init() {
	//get the auth and encypt keys from app.yaml
	authKey := strings.TrimSpace(os.Getenv("SESSION_AUTH_KEY"))
	if len(authKey) != authKeyLength {
		initError = ErrAuthKeyWrongSize
		return
	}

	encryptKey := strings.TrimSpace(os.Getenv("SESSION_ENCRYPT_KEY"))
	if len(encryptKey) != encryptKeyLength {
		initError = ErrEncyptKeyWrongSize
		initErrorVal = len(encryptKey)
		return
	}

	//init the session store
	s := sessions.NewCookieStore(
		[]byte(authKey),
		[]byte(encryptKey),
	)

	//set session options
	s.Options = options

	//store sessions to global variable
	store = s

	//done
	return
}

//CheckInit makes sure no errors occured during init()
//since init() cannot return errors and we need to make sure init() completed successfully
func CheckInit(r *http.Request) error {
	c := appengine.NewContext(r)

	if initError != nil {
		log.Errorf(c, "%v", "Error during session initialization.")
		log.Errorf(c, "%v", initError)
		return initError
	}

	return nil
}

//Get gets an existing session for a request or creates a new session if none exists
//the field "IsNew" of the returned struct will be true if this session was just created
func Get(r *http.Request) *sessions.Session {
	session, _ := store.Get(r, sessionCookieName)
	return session
}

//AddValue adds a key value pair to a session
//don't forgot to save the session after adding values to it!
//this doesn't save automatically in case you are adding lots of new values to a session...b/c saving after every add would be pointless instead of just saving once
func AddValue(session *sessions.Session, key string, value interface{}) {
	session.Values[key] = value
	return
}

//Save saves any new session data to an existing session
//write the new values to it (after using AddValue)
func Save(session *sessions.Session, w http.ResponseWriter, r *http.Request) {
	session.Save(r, w)
	return
}

//Destroy deletes a session for a request
//this logs a user out if they were logged in since middleware.Auth will no longer be able to validate the user
func Destroy(w http.ResponseWriter, r *http.Request) {
	s := Get(r)

	s.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   false,
	}
	s.Save(r, w)
	return
}

//ExtendExpiration pushes out the expiration of the session cookie to a further time
//this is done to keep a user logged in automatically if they use the app frequently
func ExtendExpiration(session *sessions.Session, w http.ResponseWriter, r *http.Request) {
	session.Options = options
	session.Save(r, w)
	return
}

//GetUsername gets the username we have stored in a session
func GetUsername(r *http.Request) string {
	s := Get(r)
	return s.Values["username"].(string)
}

//GetUserID gets the user ID we have stored in a session
func GetUserID(r *http.Request) int64 {
	s := Get(r)
	return s.Values["user_id"].(int64)
}
