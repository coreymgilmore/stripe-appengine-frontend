package sessionutils

import (
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/sessions"
)

const (
	sessionCookieName   = "session_id"
	sessionCookieDomain = "."

	//PATH TO FILES WHERE KEYS ARE STORED IN TEXT
	//keys are stored in files instead of code so they are easily changed
	//session requires two keys, authentication and encryption, for session token
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
	//STORAGE FOR AUTH AND ENCRYPTION KEYS
	authKey    []byte
	encryptKey []byte

	//GLOBAL STORE FOR SESSION DATA
	Store *sessions.CookieStore

	//OPTIONS FOR SESSIONS
	//standarized
	//cookie for session expires in 7 days
	options = &sessions.Options{
		Domain:   sessionCookieDomain,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 7,
		HttpOnly: false,
		Secure:   false,
	}

	//INIT FUNC ERRORS
	initError             error
	ErrAuthKeyWrongSize   = errors.New("Secure session auth key 'session-auth-key.txt' is not the correct length. Must by 64 bytes long.")
	ErrEncyptKeyWrongSize = errors.New("Secure session encrypt key 'session-encrypt-key.txt' is not the correct length. Must be 32 bytes long.")
)

//*********************************************************************************************************************************

//INITIALIZE THE SESSION STORAGE
//set auth and encrypt keys so that session id stored in cookie must come from this server
//also makes the session id not human readable
//the auth and encrypt keys are read from files b/c these files are more easily edited than editing code
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

	//assign to global variables
	authKey = aKey
	encryptKey = eKey

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

//GET A SESSION
//get an existing session from cookie (if it exists)
//otherwise creates a new session
//the field IsNew will be true if this session was just created
func Get(r *http.Request) *sessions.Session {
	session, _ := Store.Get(r, sessionCookieName)
	return session
}

//ADD VALUES TO A SESSION
//do not forgot to save the session after adding values to it
func AddValue(session *sessions.Session, key string, value interface{}) {
	session.Values[key] = value
	return
}

//SAVE A SESSION
//write the new values to it
//must be done after adding values to a session
func Save(session *sessions.Session, w http.ResponseWriter, r *http.Request) {
	session.Save(r, w)
	return
}

//DELETE A SESSION
//helpful for when logging a user out
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

//EXTEND EXPIRATION OF SESSION COOKIE
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
	if len(authKey) != authKeyLength {
		return ErrAuthKeyWrongSize
	}
	if len(encryptKey) != encyptKeyLength {
		return ErrEncyptKeyWrongSize
	}

	//session initialized successfully
	return nil
}
