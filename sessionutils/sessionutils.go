/*
Package sessionutils implements functionality to more easily deal with sessions for users.
This wraps around gorilla/sessions to make code clearer and more usable.

Session data stores user authentication data in a cookie. The cookie value is encrypted and authenticated via gorilla/sessions.
*/

package sessionutils

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/sessions"
)

const (
	//default cookie info
	sessionCookieName   = "session_id"
	sessionCookieDomain = "."

	//path to files where keys are stored in text
	//keys are stored in files instead of code so they are more easily changed
	//session requires two keys, one for each: authentication and encryption, for session token
	//these keys are read from files and are not published (ignored with .gitignore)
	//these files must exist for app to boot and initialize
	authKeyPath    = "config/session-auth-key.txt"
	encryptKeyPath = "config/session-encrypt-key.txt"

	//keys are a fixed and required size
	//this is the strongest settings as defined by gorilla/sessions
	authKeyLength   = 64
	encyptKeyLength = 32
)

var (
	//storage for auth and encryption keys
	authKey    []byte
	encryptKey []byte

	//global store for session data
	Store *sessions.CookieStore

	//options for sessions
	//standarized
	//cookie for session expires in 7 days
	options = &sessions.Options{
		Domain:   sessionCookieDomain,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 7,
		HttpOnly: false,
		Secure:   false,
	}

	//init func errors
	initError             error
	ErrAuthKeyWrongSize   = errors.New("Secure session auth key 'session-auth-key.txt' is not the correct length. Must by 64 bytes long.")
	ErrEncyptKeyWrongSize = errors.New("Secure session encrypt key 'session-encrypt-key.txt' is not the correct length. Must be 32 bytes long.")
)

//Init initializes the session store
//this reads and sets the auth and encryption keys for session cookie
//throw errors so app is not usable if auth or encrypt keys are missing
func Init() error {
	//get the auth and encypt keys from files
	aKey, err0 := ioutil.ReadFile(authKeyPath)
	eKey, err1 := ioutil.ReadFile(encryptKeyPath)
	if err0 != nil {
		initError = err0
		return err0
	} else if err1 != nil {
		initError = err1
		return err1
	}

	//assign to package variables
	authKey = bytes.TrimSpace(aKey)
	encryptKey = bytes.TrimSpace(eKey)

	//init the session store
	s := sessions.NewCookieStore(
		authKey,
		encryptKey,
	)

	//set session options
	s.Options = options

	//store sessions to global variable
	Store = s

	//done
	return nil
}

//Get gets an existing session for a request or creates a new session if none exists
//the field "IsNew" will be true if this session was just created
func Get(r *http.Request) *sessions.Session {
	session, _ := Store.Get(r, sessionCookieName)
	return session
}

//AddValue adds a key value pair to a session
//do not forgot to save the session after adding values to it
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

//CheckSession makes sure the session auth and encrypt keys are valid and no errors occured while setting up the session store
func CheckSession() error {
	//check that Init() did not throw any errors
	if initError != nil {
		return initError
	}

	//check that auth key is correct length
	if len(authKey) != authKeyLength {
		return ErrAuthKeyWrongSize
	}
	if len(encryptKey) != encyptKeyLength {
		return ErrEncyptKeyWrongSize
	}

	//session initialized successfully
	return nil
}
