package users

import (
	"net/http"
	"errors"
	"fmt"
	"strconv"
	
	"appengine"
	"appengine/datastore"
	"appengine/memcache"

	"github.com/coreymgilmore/pwds"
	"github.com/coreymgilmore/timestamps"
	
	"sessionutils"
	"output"
	"memcacheutils"
	"templates"
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
	ErrSessionMismatch = 		errors.New("sessionMismatch")
	ErrCannotUpdateSelf = 		errors.New("cannotUpdateYourself")
	ErrCannotUpdateSuperAdmin = errors.New("cannotUpdateSuperAdmin")
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
		notificationPage(w, "panel-danger", "Error", "The admin user already exists.", "btn-default", "/", "Go Back")
		return
	}

	//get form values
	pass1 := r.FormValue("password1")
	pass2 := r.FormValue("password2")

	//make sure they match
	if doStringsMatch(pass1, pass2) == false {
		notificationPage(w, "panel-danger", "Error", "The passwords id not match.", "btn-default", "/setup/", "Try Again")
		return
	}

	//make sure the password is long enough
	if len(pass1) < MIN_PASSWORD_LENGTH {
		notificationPage(w, "panel-danger", "Error", "The password you provided is too short. It must me at least " + strconv.FormatInt(MIN_PASSWORD_LENGTH, 10) + " characters.", "btn-default", "/setup/", "Try Again")
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
	completeKey, err := saveUser(c, incompleteKey, u)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	//save user to session
	//this is how we authenticate users that are already signed in
	//the user is automatically logged in as the administrator user
	session := sessionutils.Get(r)
	if session.IsNew == false {
		notificationPage(w, "panel-danger", "Error", "An error occured while saving the admin user. Please clear your cookies and restart your browser.", "btn-default", "/setup/", "Try Again")
		return
	}
	sessionutils.AddValue(session, "username", ADMIN_USERNAME)
	sessionutils.AddValue(session, "user_id", completeKey.IntID())
	sessionutils.Save(session, w, r)

	//show user main page
	http.Redirect(w, r, "/main/", http.StatusFound)
	return
}

//LOGIN
func Login(w http.ResponseWriter, r *http.Request) {
	//get form values
	username := r.FormValue("username")
	password := r.FormValue("password")

	//get user data
	c := 				appengine.NewContext(r)
	id, data, err := 	exists(c, username)
	if err == ErrUserDoesNotExist {
		notificationPage(w, "panel-danger", "Cannot Log In", "The username you provided does not exist.", "btn-default", "/", "Try Again")
		return
	}

	//is user allowed access
	if AllowedAccess(data) == false {
		notificationPage(w, "panel-danger", "Cannot Log In", "You are not allowed access. Please contact an administrator.", "btn-default", "/", "Go Back")
		return
	}

	//validate password
	_, err = pwds.Verify(password, data.Password)
	if err != nil {
		notificationPage(w, "panel-danger", "Cannot Log In", "The password you provided is invalid.", "btn-default", "/", "Try Again")
		return
	}

	//user validated
	//save session data
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
		output.Error(ErrUserAlreadyExists, "This username already exists. Please choose a different username.", w)
		return
	}

	//make sure passwords match
	if doStringsMatch(password1, password2) == false {
		output.Error(ErrPasswordsDoNotMatch, "The passwords you provided to not match.", w)
		return
	}

	//make sure password is long enough
	if len(password1) < MIN_PASSWORD_LENGTH {
		output.Error(ErrPasswordTooShort, "The password you provided is too short. It must be at least " + strconv.FormatInt(MIN_PASSWORD_LENGTH, 10) + " characters.", w)
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
	_, err = 			saveUser(c, incompleteKey, u)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	//clear list of users saved in memcache since a new user was added
	memcacheutils.Delete(c, LIST_OF_USERS_KEYNAME)

	//respond to client with success message
	output.Success("addNewUser", nil, w)
	return
}

//GET LIST OF ALL USERS
func GetAll(w http.ResponseWriter, r *http.Request) {
	//check if list of users is saved in memcache
	result := 	make([]userList, 0, 5)
	c := 		appengine.NewContext(r)
	_, err := 	memcache.Gob.Get(c, LIST_OF_USERS_KEYNAME, &result)
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
		memcacheutils.Save(c, LIST_OF_USERS_KEYNAME, idsAndNames)

		//return data to clinet
		output.Success("userList", idsAndNames, w)
		return
	
	} else if err != nil {
		output.Error(err, "Unknown error retreiving list of users.", w)
		return
	}

	return
}

//GET ONE USER'S DATA
func GetOne(w http.ResponseWriter, r *http.Request) {
	//get user id from form value
	userId := 		r.FormValue("userId")
	userIdInt, _ :=	strconv.ParseInt(userId, 10, 64)

	//get user data
	//looks in memcache and in datastore
	c := 			appengine.NewContext(r)
	data, err := 	Find(c, userIdInt)
	if err != nil {
		output.Error(err, "Cannot look up user data.", w)
		return
	}

	//return user data
	output.Success("findUser", data, w)
	return
}

//UPDATE A USER'S PASSWORD
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
		output.Error(ErrPasswordTooShort, "The password you provided is too short. It must be at least " + strconv.FormatInt(MIN_PASSWORD_LENGTH, 10) + " characters.", w)
		return
	}

	//hash the password
	hashedPwd := pwds.Create(password1)

	//get user data
	c := 				appengine.NewContext(r)
	userData, err := 	Find(c, userIdInt)
	if err != nil {
		output.Error(err, "Error while retreiving user data to update user's password.", w)
		return
	}

	//set new password
	userData.Password = hashedPwd

	//clear memcache for this userID & username
	err = 	memcacheutils.Delete(c, userId)
	err1 := memcacheutils.Delete(c, userData.Username)
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
	_, err = saveUser(c, fullKey, userData)
	if err != nil {
		output.Error(err, "Error saving user to database after password change.", w)
		return
	}

	//done
	output.Success("userChangePassword", nil, w)
	return
}

//UPDATE A USER'S PERMISSIONS
func UpdatePermissions(w http.ResponseWriter, r *http.Request) {
	//gather form values
	userId := 			r.FormValue("userId")
	userIdInt, _ := 	strconv.ParseInt(userId, 10, 64)
	addCards, _ := 		strconv.ParseBool(r.FormValue("addCards"))
	removeCards, _ := 	strconv.ParseBool(r.FormValue("removeCards"))
	chargeCards, _ := 	strconv.ParseBool(r.FormValue("chargeCards"))
	viewReports, _ := 	strconv.ParseBool(r.FormValue("reports"))
	isAdmin, _ := 		strconv.ParseBool(r.FormValue("admin"))
	isActive, _ := 		strconv.ParseBool(r.FormValue("active"))

	//check if the logged in user is an admin
	//user updating another user's permission must be an admin
	//failsafe/second check since non-admins would not see the settings panel anyway
	session := sessionutils.Get(r)
	if session.IsNew {
		output.Error(ErrSessionMismatch, "An error occured. Please log out and log back in.", w)
		return
	}

	//get user data to update
	c := 				appengine.NewContext(r)
	userData, err := 	Find(c, userIdInt)
	if err != nil {
		output.Error(err, "We could not retrieve this user's information. This user could not be updates.", w)
		return
	}

	//check if the logged in user is trying to update their own permissions
	//you cannot edit your own permissions no matter what
	if session.Values["username"].(string) == userData.Username {
		output.Error(ErrCannotUpdateSelf, "You cannot edit your own permissions. Please contact another administrator.", w)
		return
	}

	//check iF user is editing the super admin user
	if userData.Username == ADMIN_USERNAME {
		output.Error(ErrCannotUpdateSuperAdmin, "You cannot update the 'administrator' user. The account is locked.", w)
		return
	}

	//update the user
	userData.AddCards = 		addCards
	userData.RemoveCards = 		removeCards
	userData.ChargeCards = 		chargeCards
	userData.ViewReports = 		viewReports
	userData.Administrator = 	isAdmin
	userData.Active = 			isActive

	//clear memcache
	err = 	memcacheutils.Delete(c, userId)
	err1 := memcacheutils.Delete(c, userData.Username)
	if err != nil {
		output.Error(err, "Error clearing cache for user id.", w)
		return
	} else if err1 != nil {
		output.Error(err1, "Error clearing cache for username.", w)
		return
	}
	
	//generate complete key for user
	completeKey := getUserKeyFromId(c, userIdInt)

	//resave user
	//saves to datastore and memcache
	//save user
	_, err = saveUser(c, completeKey, userData)
	if err != nil {
		output.Error(err, "Error saving user to database after updating permission.", w)
		return
	}

	//done
	output.Success("userUpdatePermissins", nil, w)
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
	return datastore.NewKey(c, DATASTORE_KIND, "", id, nil)
}

//**********************************************************************
//FUNCS

//CHECK IF ADMIN USER EXISTS
//otherwise redirect user to create admin user
//this should only happen on the first time the app starts
//admin user is required to log in and create other users
func DoesAdminExist(r *http.Request) error {
	//query for admin user
	var user []User
	c := 			appengine.NewContext(r)
	q := 			datastore.NewQuery(DATASTORE_KIND).Filter("Username = ", ADMIN_USERNAME).KeysOnly()
	keys, err := 	q.GetAll(c, &user)
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
func saveUser(c appengine.Context, key *datastore.Key, user User) (*datastore.Key, error) {
	//save to datastore
	completeKey, err := datastore.Put(c, key, &user)
	if err != nil {
		return completeKey, err
	}

	//save user to memcache
	mKey := strconv.FormatInt(completeKey.IntID(), 10)
	err = 	memcacheutils.Save(c, mKey, user)
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
//GET USER DATA

//GET DATA BY DATASTORE ID
//check for data in memcache first, then datastore
//add to memcache if data does not exist
//userId is the IntID of an entity key
func Find(c appengine.Context, userId int64) (User, error) {
	//memcache
	var r User
	userIdStr := 	strconv.FormatInt(userId, 10)
	_, err := 		memcache.Gob.Get(c, userIdStr, &r)
	
	if err == nil {
		//data found in memcache
		return r, nil
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
		memcacheutils.Save(c, userIdStr, userData)

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

//SHOW THE NOTIFICATION PAGES
//same as pages.notificationPage but have to have separate function b/c of dependecy circle
func notificationPage(w http.ResponseWriter, panelType, title string, err interface{}, btnType, btnPath, btnText string) {
	templates.Load(w, "notifications", templates.NotificationPage{panelType, title, err, btnType, btnPath, btnText})
	return
}
