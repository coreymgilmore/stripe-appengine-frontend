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
	"output"
)

const (
	ADMIN_USERNAME = 		"administrator"
	DATASTORE_KIND = 		"users"
	MIN_PASSWORD_LENGTH = 	8
	LIST_OF_USERS_KEYNAME = "list-of-users"
)

var (
	ErrAdminDoesNotExist = 		errors.New("adminUserDoesNotExist")
	ErrUserDoesNotExist = 		errors.New("userDoesNotExist")
	ErrUserAlreadyExists = 		errors.New("userAlreadyExists")
	ErrPasswordsDoNotMatch = 	errors.New("passwordsDoNotMatch")
	ErrPasswordTooShort = 		errors.New("passwordTooShort")
	ErrNotAdmin = 				errors.New("userIsNotAnAdmin")
)

type User struct{
	Username 		string 		`json:"username"`
	Password 		string 		`json:"-"`
	AddCards 		bool 		`json:"add_cards"`
	RemoveCards 	bool 		`json:"remove_cards"`
	ChargeCards 	bool 		`json:"charge_cards"`
	ViewReports 	bool 		`json:"view_reports"`
	Administrator 	bool 		`json:"is_admin"`
	Active 			bool 		`json:"is_active"`
	Created 		string 		`json:"datetime_created"`
}

type userList struct {
	Username 		string 		`json:"username"`
	Id 				int64 		`json:"id"`
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
	if doStringsMatch(pass1, pass2) == false {
		templates.Load(w, "notifications", templates.NotificationPage{"panel-danger", "Error", "The passwords you provided did not match.", "btn-default", "/setup/", "Try Again"})
		return
	}

	//make sure the password is long enough
	if len(pass1) < MIN_PASSWORD_LENGTH {
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

//ADD A NEW USER
//gathers data from ajax call
//does some validation
//creates and saved the user to datastore and saves user to memcache by IntID
func Add(w http.ResponseWriter, r *http.Request) {
	//get form values
	username := 		r.FormValue("username")
	password1 := 		r.FormValue("password1")
	password2 := 		r.FormValue("password2")
	addCards, _ := 		strconv.ParseBool(r.FormValue("addCards"))
	removeCards, _ := 	strconv.ParseBool(r.FormValue("removeCards"))
	chargeCards, _ := 	strconv.ParseBool(r.FormValue("chargeCards"))
	viewReports, _ := 	strconv.ParseBool(r.FormValue("reports"))
	isAdmin, _ := 		strconv.ParseBool(r.FormValue("admin"))
	isActive, _ := 		strconv.ParseBool(r.FormValue("active"))

	//check if this user already exists
	c := appengine.NewContext(r)
	_, _, err := exists(c, username)
	if err == nil {
		//user already exists
		//notify client
		output.Error(ErrUserAlreadyExists, "You cannot create a user with this username because this user already exists.", w)
		return
	}

	//make sure passwords match
	if doStringsMatch(password1, password2) == false {
		output.Error(ErrPasswordsDoNotMatch, "The passwords you provided to not match.", w)
		return
	}

	//make sure password is long enough
	if len(password1) < MIN_PASSWORD_LENGTH {
		output.Error(ErrPasswordTooShort, "The password you provided is too short. It must be at least " + strconv.FormatInt(MIN_PASSWORD_LENGTH, 10) + " characters long.", w)
		return
	}

	//hash the password
	hashedPwd := pwds.Create(password1)

	//create the user
	u := User{
		Username: 		username,
		Password: 		hashedPwd,
		AddCards: 		addCards,
		RemoveCards: 	removeCards,
		ChargeCards: 	chargeCards,
		ViewReports: 	viewReports,
		Administrator: 	isAdmin,
		Active: 		isActive,
		Created: 		timestamps.ISO8601(),	
	}

	//save to datastore
	incompleteKey := 	createNewUserKey(c)
	_, err = 			saveNewUser(c, incompleteKey, u)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	//clear list of users saved in memcache since a new user was added
	memcacheDelete(c, LIST_OF_USERS_KEYNAME)

	//respond to client with success message
	output.Success("addNewUser", nil, w)
	return
}

//GET LIST OF ALL USERS
//return object of IntID: username to be used in building select options
func GetAll(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	//check if current logged in user is an admin
	//only admins can get the list of users
	session := 			sessionutils.Get(r)
	userId := 			session.Values["user_id"].(int64)
	userData, err := 	Find(c, userId)
	if err != nil {
		output.Error(err, "Error verifying if you are an administrator while getting list of users.", w)
		return
	}
	if userData.Administrator == false {
		//user is not an admin
		//cannot access list of users
		output.Error(ErrNotAdmin, "You are not an administrator therefore you cannot access the list of users.", w)
		return
	}

	//check if list of users is saved in memcache
	result := 	make([]userList, 0, 5)
	_, err = 	memcache.Gob.Get(c, LIST_OF_USERS_KEYNAME, &result)
	if err == nil {
		//return results
		output.Success("userList-cached", result, w)
		return
	}
	
	//list of users not found in memcache
	//look up users in datastore
	if err == memcache.ErrCacheMiss {
		q := 			datastore.NewQuery(DATASTORE_KIND).Order("Username").Project("Username")
		users := 		make([]User, 0, 5)
		keys, err := 	q.GetAll(c, &users)
		if err != nil {
			output.Error(err, "Error retrieving list of users from datastore.", w)
			return
		}

		//build result
		idsAndNames := make([]userList, 0, 5)
		for i, r := range users {
			x := userList{
				Username: 	r.Username,
				Id: 		keys[i].IntID(),
			}

			idsAndNames = append(idsAndNames, x)
		}

		//save the list of users to memcache
		//ignore errors since we still retrieved the data
		memcacheSave(c, LIST_OF_USERS_KEYNAME, idsAndNames)

		//return data to user
		output.Success("userList", idsAndNames, w)
		return
	
	} else if err != nil {
		output.Error(err, "Unknown error retreiving list of users.", w)
		return
	}

	return;
}

//CHANGE A USER'S PASSWORD
func ChangePwd(w http.ResponseWriter, r *http.Request) {
	//gather inputs
	userId := 		r.FormValue("userId")
	userIdInt, _ := strconv.ParseInt(userId, 10, 64)
	password1 := 	r.FormValue("pass1")
	password2 := 	r.FormValue("pass2")

	//make sure passwords match
	if doStringsMatch(password1, password2) == false {
		output.Error(ErrPasswordsDoNotMatch, "The passwords you provided to not match.", w)
		return
	}

	//make sure password is long enough
	if len(password1) < MIN_PASSWORD_LENGTH {
		output.Error(ErrPasswordTooShort, "The password you provided is too short. It must be at least " + strconv.FormatInt(MIN_PASSWORD_LENGTH, 10) + " characters long.", w)
		return
	}

	//hash the password
	hashedPwd := pwds.Create(password1)

	//get user data
	c := appengine.NewContext(r)
	userData, err := Find(c, userIdInt)
	if err != nil {
		output.Error(err, "Error while retreiving user data to update user's password.", w)
		return
	}

	//set new password
	userData.Password = hashedPwd

	//clear memcache for this userID & username
	err = 	memcacheDelete(c, userId)
	err1 := memcacheDelete(c, userData.Username)
	if err != nil {
		output.Error(err, "Error clearing cache for user id.", w)
		return
	} else if err1 != nil {
		output.Error(err1, "Error clearing cache for username.", w)
		return
	}

	//generate full datastore key for user
	fullKey := getUserKeyFromId(c, userIdInt)

	//save user
	_, err = saveNewUser(c, fullKey, userData)
	if err != nil {
		output.Error(err, "Error saving user to database after password change.", w)
		return
	}

	//done
	output.Success("userUpdate", nil, w)
	return
}

//GET ONE USER'S DATA
func GetOne(w http.ResponseWriter, r *http.Request) {
	//get user id from form value
	userId := 		r.FormValue("userId")
	userIdInt, _ :=	strconv.ParseInt(userId, 10, 64)

	//get user data
	//looks in memcache and in datastore
	data, err := 	Find(c, userIdInt)
	if err != nil {
		output.Error(err, "Cannot look up user data.", w)
		return
	}

	//return user data
	output.Success("findUser", data, w)
	return
}


//**********************************************************************
//DATASTORE KEYS

//CREATE INCOMPLETE KEY TO SAVE NEW USER
func createNewUserKey(c appengine.Context) *datastore.Key {
	return datastore.NewIncompleteKey(c, DATASTORE_KIND, nil)
}

//CREATE COMPLETE KEY FOR USER
//get the full complete key from just the ID of a key
func getUserKeyFromId(c appengine.Context, id int64) *datastore.Key {
	key := datastore.NewKey(c, DATASTORE_KIND, "", id, nil)
	return key
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

//IS USER ALLOWED ACCESS TO THIS APP
func AllowedAccess(data User) bool {
	return data.Active
}

//CHECK IF TWO STRINGS MATCH
func doStringsMatch(string1, string2 string) bool {
	if string1 == string2 {
		return true
	}

	return false
}

//**********************************************************************
//MEMCACHE

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

//DELETE FROM MEMCACHE
func memcacheDelete(c appengine.Context, key string) error {
	err := memcache.Delete(c, key)
	if err == memcache.ErrCacheMiss {
		//key does not exist
		//this is not an error
		return nil
	} else if err != nil {
		return err
	}

	//delete successful
	return nil
}

//**********************************************************************
//GET USER DATA

//GET DATA BY DATASTORE ID
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

//CHECK IF A USER EXISTS
//also, get user data by username
//returns error if a user by the username 'username' does not exist
//error returned when a user cannot be found
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
