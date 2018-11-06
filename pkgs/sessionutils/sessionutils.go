/*
Package sessionutils implements functionality to more easily deal with sessions for users.
This wraps around gorilla/sessions to make code clearer and more usable.

Session data stores user authentication data in a cookie.
The cookie value is encrypted and authenticated via gorilla/sessions.
*/
package sessionutils

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
)

//config is the set of configuration options for the session store
//this struct is used when SetConfig is run in package main init()
type config struct {
	SessionAuthKey    string //a 64 character long string
	SessionEncryptKey string //a 32 character long string
	SessionLifetime   int    //number of days a user will remain logged in for
	CookieDomain      string //domain to serve cookies on
}

//Config is a copy of the config struct with some defaults set
var Config = config{
	SessionAuthKey:    "",
	SessionEncryptKey: "",
	SessionLifetime:   7,   //default value in case this doesn't get set before calling SetConfig()
	CookieDomain:      "/", //"." is any domain
}

//this is the required sizes of the SessionAuthKey and SessionEncryptKey
//these values are the strongest possible per gorilla/session
const (
	authKeyLength    = 64
	encryptKeyLength = 32
)

//configuration errors
var (
	errAuthKeyWrongSize       = errors.New("session: Auth key is invalid. Provide an auth key in app.yaml that is exactly 64 bytes long")
	errEncyptKeyWrongSize     = errors.New("session: Encrypt key is invalid. Provide an encrypt key in app.yaml that is exactly 32 bytes long")
	errInvalidSessionLifetime = errors.New("session: Lifetime must be an integer greater than 0")
)

//sessionCookieName is the name of the cookie saved to clients that stores our session information
var sessionCookieName = "cc_app_session_id"

//store is a variable for dealing with session data
var store *sessions.CookieStore

//options for session store
var options = &sessions.Options{
	Domain: ".",
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

//SetConfig saves the configuration for the session store and starts the session store
func SetConfig(c config) error {
	//validate config options
	authKey := strings.TrimSpace(c.SessionAuthKey)
	if len(authKey) != authKeyLength {
		return errAuthKeyWrongSize
	}

	encryptKey := strings.TrimSpace(c.SessionEncryptKey)
	if len(encryptKey) != encryptKeyLength {
		return errEncyptKeyWrongSize
	}

	//initialize the session store
	s := sessions.NewCookieStore(
		[]byte(authKey),
		[]byte(encryptKey),
	)

	//make sure session lifetime is a valid value
	if c.SessionLifetime < 1 {
		log.Fatalln("Session lifetime is invalid.  It must be an integer greater than 0.")
		return errInvalidSessionLifetime
	}

	//set session options
	options.MaxAge = 60 * 60 * 24 * int(c.SessionLifetime)
	options.Domain = c.CookieDomain
	s.Options = options

	//save session store for use later
	store = s

	//save config to package variable
	Config = c

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
