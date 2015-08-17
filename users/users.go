package users

import (
	"net/http"
	"errors"
	"fmt"
	"github.com/coreymgilmore/pwds"
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"templates"
	"github.com/coreymgilmore/timestamps"
	"strconv"
	"sessionutils"
)

const (
	ADMIN_USERNAME = "admin@example.com"
	DATASTORE_KIND = "users"
)

var (
	ErrAdminDoesNotExist = 	errors.New("Administrator user does not exist.")
	ErrUserDoesNotExist = 	errors.New("User does not exist")
)

type User struct{
	Username 		string
	Password 		string
	AddCards 		bool
	RemoveCards 	bool
	ChargeCards 	bool
	ViewReports 	bool
	Administrator 	bool
	Active 			bool
	Created 		string
}

//**********************************************************************
//HANDLE HTTP REQUESTS

//SAVE THE INITIAL ADMIN USER
func CreateAdmin(w http.ResponseWriter, r *http.Request) {
	//make sure the admin user doesnt already exist
	err := DoesAdminExist(r)
	if err == nil {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Error", "The admin user already exists. You cannot create it again.", "btn-default", "/", "Go Back"})
		return
	}

	//get form values
	pass1 := r.FormValue("password1")
	pass2 := r.FormValue("password2")

	//make sure they match
	if pass1 != pass2 {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Error", "The passwords you provided did not match.", "btn-default", "/setup/", "Try Again"})
		return
	}

	//make sure the password is long enough
	if len(pass1) < 8 {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Error", "The password you provided is not long enough.", "btn-default", "/setup/", "Try Again"})
		return
	}

	//hash the password
	hashedPwd := pwds.Create(pass1)

	//create the user
	u := User{
		Username: 		ADMIN_USERNAME,
		Password: 		hashedPwd,
		AddCards: 		true,
		RemoveCards: 	true,
		ChargeCards: 	true,
		ViewReports: 	true,
		Administrator: 	true,
		Active: 		true,
		Created: 		timestamps.ISO8601(),	
	}

	//save to datastore
	c := 				appengine.NewContext(r)
	incompleteKey := 	createNewUserKey(c)
	completeKey, err := saveNewUser(c, incompleteKey, u)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	//save user to session
	//this is how we authenticate users that are already signed in
	session := sessionutils.Get(r)
	if session.IsNew == false {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Error", "An error occured while trying to save the admin user. Please clear your cookies and restart your browser.", "btn-default", "/setup/", "Try Again"})
		return
	}
	sessionutils.AddValue(session, "username", ADMIN_USERNAME)
	sessionutils.AddValue(session, "user_id", completeKey.IntID())
	sessionutils.Save(session, w, r)

	//show user main page
	http.Redirect(w, r, "/main/", http.StatusFound)
	return
}

//LOGIN A USER
//from root page
func Login(w http.ResponseWriter, r *http.Request) {
	//get form values
	username := r.FormValue("username")
	password := r.FormValue("password")

	//get user data
	c := 				appengine.NewContext(r)
	id, data, err := 	exists(c, username)
	if err == ErrUserDoesNotExist {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Cannot Log In", "The username you provided does not exist.", "btn-default", "/", "Go Back"})
		return
	}

	//is user allowed access
	if AllowedAccess(data) == false {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Cannot Log In", "You are not allowed access. Please contact an administrator.", "btn-default", "/", "Go Back"})
		return
	}

	//validate password
	_, err = pwds.Verify(password, data.Password)
	if err != nil {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Cannot Log In", "The password you provided is invalid.", "btn-default", "/", "Go Back"})
		return
	}

	//user validated
	//save session token
	session := sessionutils.Get(r)
	if session.IsNew == false {
		sessionutils.Destroy(w, r)
		session = sessionutils.Get(r)
	}

	sessionutils.AddValue(session, "username", username)
	sessionutils.AddValue(session, "user_id", id)
	sessionutils.Save(session, w, r)

	//show user main page
	http.Redirect(w, r, "/main/", http.StatusFound)
	return
}

//LOGOUT
func Logout(w http.ResponseWriter, r *http.Request) {
	//destroy session
	sessionutils.Destroy(w, r)

	//redirect to root page
	http.Redirect(w, r, "/", http.StatusFound)
	return
}

//**********************************************************************
//FUNCS

//CHECK IF ADMIN USER EXISTS
//otherwise redirect user to create admin user
//this should only happen on the first time the app starts
//admin user is required to log in and create other users
func DoesAdminExist(r *http.Request) error {
	c := appengine.NewContext(r)

	//query for admin user
	var user []User
	q := datastore.NewQuery(DATASTORE_KIND).Filter("Username=", ADMIN_USERNAME).KeysOnly()
	keys, err := q.GetAll(c, &user)
	if err != nil {
		return err
	}

	//check if a result was found
	if len(keys) == 0 {
		return ErrAdminDoesNotExist
	}

	//admin user exists
	return nil
}

//CREATE KEY FOR NEW USER
func createNewUserKey(c appengine.Context) *datastore.Key {
	return datastore.NewIncompleteKey(c, DATASTORE_KIND, nil)
}

//SAVE A USER TO THE DATASTORE
//input key is an incomplete key
//returned key is a complete key...use this to save session data
func saveNewUser(c appengine.Context, key *datastore.Key, user User) (*datastore.Key, error) {
	//save user
	completeKey, err := datastore.Put(c, key, &user)
	if err != nil {
		return key, err
	}

	//save user to memcache
	memcacheKey := strconv.FormatInt(completeKey.IntID(), 10)
	err = memcacheSave(c, memcacheKey, user)
	if err != nil {
		return completeKey, err
	}

	//done
	return completeKey, nil
}

//SAVE TO MEMCACHE
//key is actually an int as a string (the intID of a key)
func memcacheSave(c appengine.Context, key string, value interface{}) error {
	//build memcache item to store
	item := &memcache.Item{
		Key: 	key,
		Object: value,
	}

	//save
	err := memcache.Gob.Set(c, item)
	if err != nil {
		return err
	}

	//done
	return nil
}

//GET USER DATA
//check for data in memcache first, then datastore
//add to memcache if data does not exist
//userId is the IntID of an entity key
func Find(c appengine.Context, userId int64) (User, error) {
	//memcache
	var memcacheResult User
	userIdStr := 	strconv.FormatInt(userId, 10)
	_, err := 		memcache.Gob.Get(c, userIdStr, &memcacheResult)
	if err == nil {
		//data found in memcache
		return memcacheResult, nil
	} else if err == memcache.ErrCacheMiss {
		//data not found in memcache
		//look in datastore
		key := 		getUserKeyFromId(c, userId)
		q := 		datastore.NewQuery(DATASTORE_KIND).Filter("__key__ =", key).Limit(1)
		result := 	make([]User, 0, 1)
		_, err := 	q.GetAll(c, &result)
		if err != nil {
			return User{}, err
		}

		//one result
		userData := result[0]

		//data found
		//save to memcache
		//ignore errors since we still found the data
		memcacheSave(c, userIdStr, userData)

		//done
		return userData, nil
	} else {
		return User{}, err
	}
}

//CREATE USER KEY FROM ID
//get the full complete key from just the ID of a key
func getUserKeyFromId(c appengine.Context, id int64) *datastore.Key {
	key := datastore.NewKey(c, DATASTORE_KIND, "", id, nil)
	return key
}

//IS USER ALLOWED ACCESS TO THIS APP
func AllowedAccess(data User) bool {
	return data.Active
}

//CHECK IF A USER EXISTS BY USERNAME
func exists(c appengine.Context, username string) (int64, User, error) {
	q := 		datastore.NewQuery(DATASTORE_KIND).Filter("Username = ", username).Limit(1)
	result := 	make([]User, 0, 1)
	keys, _ := 	q.GetAll(c, &result)

	//user was not found
	if len(keys) == 0 {
		return 0, User{}, ErrUserDoesNotExist
	}

	//return user found data
	return keys[0].IntID(), result[0], nil
}
