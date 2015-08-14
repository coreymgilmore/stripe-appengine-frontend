package sessionutils

import (
	"net/http"
	"io/ioutil"

	"github.com/gorilla/sessions"
)

const (
	SESSION_COOKIE_NAME = 	"session_id"
	SESSION_COOKIE_DOMAIN = "."
	
	//gorilla/sessions requires two keys to authenticate and encrypt session data and session token (saved in cookie)
	//these are read from files so they will not be published to SVN (these files are in .gitignore)
	AUTH_KEY_PATH = 		"secrets/session-auth-key.txt"
	ENCRYPT_KEY_PATH = 		"secrets/session-encrypt-key.txt"
)

var (
	//global store for session data
	Store *sessions.CookieStore

	//setup options for sessions
	options = &sessions.Options{
		Domain: 	SESSION_COOKIE_DOMAIN,
		Path: 		"/",
		MaxAge: 	60 * 60 * 24 * 7,
		HttpOnly: 	false,
		Secure: 	false,
	}
)

//*********************************************************************************************************************************

//INITIALIZE THE SESSION STORAGE
//set auth and encrypt keys so that session id stored in cookie must come from this server
//also makes the session id not human readable
func Init() error {
	//get the auth and encypt keys from files
	authKey, err0 := 	ioutil.ReadFile(AUTH_KEY_PATH)
	encryptKey, err1 := ioutil.ReadFile(ENCRYPT_KEY_PATH)
	if err0 != nil {
		return err0
	}else if err1 != nil {
		return err1
	}

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
