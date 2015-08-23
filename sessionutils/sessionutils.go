package sessionutils

import (
	"net/http"
	"io/ioutil"
	"errors"

	"github.com/gorilla/sessions"
)

const (
	SESSION_COOKIE_NAME = 	"session_id"
	SESSION_COOKIE_DOMAIN = "."
	
	//PATH TO FILES WHERE KEYS ARE STORED IN TEXT
	//keys are stored in files instead of code so they are easily changed
	//session requires two keys, authentication and encryption, for session token
	//these keys are read from files and are not published (ignored with .gitignore)
	AUTH_KEY_PATH = 		"config/session-auth-key.txt"
	ENCRYPT_KEY_PATH = 		"config/session-encrypt-key.txt"

	//keys are a fixed and required size
	AUTH_KEY_LENGTH = 		64
	ENCRYPT_KEY_LENGTH = 	32
)

var (
	//storage for auth and encryption keys
	AUTH_KEY []byte
	ENCRYPT_KEY []byte
	
	//global store for session data
	Store *sessions.CookieStore

	//setup options for sessions
	options = &sessions.Options{
		Domain: 	SESSION_COOKIE_DOMAIN,
		Path: 		"/",
		MaxAge: 	60 * 60 * 24 * 7,	//7 days
		HttpOnly: 	false,
		Secure: 	false,
	}

	//save errors from Init() for use when checking if the session is set up correctly
	initError error

	//errors when checking Init() for errors and keys are correct size
	ErrAuthKeyWrongSize = 	errors.New("Secure session auth key ('session-auth-key.txt') is not the correct length. Must by 64 bytes long.")
	ErrEncyptKeyWrongSize = errors.New("Secure session encrypt key ('session-encrypt-key.txt') is not the correct length. Must be 32 bytes long.")
)

//*********************************************************************************************************************************

//INITIALIZE THE SESSION STORAGE
//set auth and encrypt keys so that session id stored in cookie must come from this server
//also makes the session id not human readable
func Init() error {
	//get the auth and encypt keys from files
	aKey, err0 := 	ioutil.ReadFile(AUTH_KEY_PATH)
	eKey, err1 := ioutil.ReadFile(ENCRYPT_KEY_PATH)
	if err0 != nil {
		initError = err0
		return err0
	} else if err1 != nil {
		initError = err1
		return err1
	}

	//assign to global variables
	//used to make sure cookiestore keys are correct before app can be used
	AUTH_KEY = 		aKey
	ENCRYPT_KEY = 	eKey

	//init the session store
	s := sessions.NewCookieStore(
		AUTH_KEY,
		ENCRYPT_KEY,
	)

	//set session options
	s.Options = options

	//store sessions to global variable
	Store = s

	//done
	return nil
}

//GET A SESSION
//get an existing session from cookie (if it exists)
//otherwise get a new session
func Get(r *http.Request) *sessions.Session {
	session, _ := Store.Get(r, SESSION_COOKIE_NAME)
	return session
}

//ADD VALUES TO A SESSION
func AddValue(session *sessions.Session, key string, value interface{}) {
	session.Values[key] = value
	return
}

//SAVE A SESSION
//write the new values to it
func Save(session *sessions.Session, w http.ResponseWriter, r *http.Request) {
	session.Save(r, w)
	return
}

//DELETE A SESSION
func Destroy(w http.ResponseWriter, r *http.Request) {
	s := Get(r)

	s.Options = &sessions.Options{
		Path: 		"/",
		MaxAge: 	-1,
		HttpOnly: 	false,
		Secure: 	false,
	}
	s.Save(r, w)
	return
}

//EXTEND EXPIRATION OF SESSION ID COOKIE
func ExtendExpiration(session *sessions.Session, w http.ResponseWriter, r *http.Request) {
	session.Options = options
	session.Save(r, w)
	return
}

//CHECK SESSION KEYS
//make sure they are valid options
//and that Init() did not encounter any errors
func CheckSession() error {
	//check that Init() did not throw any errors
	if initError != nil {
		return initError
	}

	//check that auth key is correct length
	if len(AUTH_KEY) != AUTH_KEY_LENGTH {
		return ErrAuthKeyWrongSize
	}
	if len(ENCRYPT_KEY) != ENCRYPT_KEY_LENGTH {
		return ErrEncyptKeyWrongSize
	}

	//session initialized successfully
	return nil
}