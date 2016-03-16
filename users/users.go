/*
This file stored the common and most used functionality for working with users.
Anything regarding a lookup in the datastore should be done through a function on this page.
This allows for a central location for functions and reduces retyping/recoding things.
*/

package users

import (
	"errors"
	"net/http"
	"strconv"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"

	"memcacheutils"
	"output"
	"templates"
)

const (
	//default "super admin" username
	//this is created the first time the app is run and no datastore data exists yet
	adminUsername = "administrator"

	//the name for the "collection" or "table" of users
	//defined once so that we can change the name in one place if needed
	datastoreKind = "users"

	//require a min password length for security concerns
	//this should be changed before any users are added
	//this seems like a "reasonable" requirement
	minPwdLength = 8

	//used in memcache for storing a list of users
	listOfUsersKey = "list-of-users"
)

var (
	ErrAdminDoesNotExist      = errors.New("adminUserDoesNotExist")
	ErrUserDoesNotExist       = errors.New("userDoesNotExist")
	ErrUserAlreadyExists      = errors.New("userAlreadyExists")
	ErrPasswordsDoNotMatch    = errors.New("passwordsDoNotMatch")
	ErrPasswordTooShort       = errors.New("passwordTooShort")
	ErrNotAdmin               = errors.New("userIsNotAnAdmin")
	ErrSessionMismatch        = errors.New("sessionMismatch")
	ErrCannotUpdateSelf       = errors.New("cannotUpdateYourself")
	ErrCannotUpdateSuperAdmin = errors.New("cannotUpdateSuperAdmin")
)

//USER DATA
//used for storing and retreiving a user from the datastore
//AddCards, RemoveCards, ChargeCards, Administrator, & Active are all permissions
//if any of these are false, the user will lose permission to do the associated task
//if Active is false, user is not allowed to use the app.
type User struct {
	Username      string `json:"username"`
	Password      string `json:"-"`
	AddCards      bool   `json:"add_cards"`
	RemoveCards   bool   `json:"remove_cards"`
	ChargeCards   bool   `json:"charge_cards"`
	ViewReports   bool   `json:"view_reports"`
	Administrator bool   `json:"is_admin"`
	Active        bool   `json:"is_active"`
	Created       string `json:"datetime_created"`
}

//LIST OF USERS
//for building select list when an admin is choosing a user to edit
//Id is a datastore IntID()
type userList struct {
	Id       int64  `json:"id"`
	Username string `json:"username"`
}

//**********************************************************************
//HANDLE HTTP REQUESTS

//GET LIST OF ALL USERS
//list of users is returned as a map of structs
//each struct has the user's datastore IntID() and the username
func GetAll(w http.ResponseWriter, r *http.Request) {
	//check if list of users is saved in memcache
	result := make([]userList, 0, 5)
	c := appengine.NewContext(r)
	_, err := memcache.Gob.Get(c, listOfUsersKey, &result)
	if err == nil {
		//return results
		output.Success("userList-cached", result, w)
		return
	}

	//list of users not found in memcache
	//look up users in datastore
	if err == memcache.ErrCacheMiss {
		q := datastore.NewQuery(datastoreKind).Order("Username").Project("Username")
		users := make([]User, 0, 5)
		keys, err := q.GetAll(c, &users)
		if err != nil {
			output.Error(err, "Error retrieving list of users from datastore.", w, r)
			return
		}

		//build result
		idsAndNames := make([]userList, 0, 5)
		for i, r := range users {
			x := userList{
				Username: r.Username,
				Id:       keys[i].IntID(),
			}

			idsAndNames = append(idsAndNames, x)
		}

		//save the list of users to memcache
		//ignore errors since we still retrieved the data
		memcacheutils.Save(c, listOfUsersKey, idsAndNames)

		//return data to clinet
		output.Success("userList", idsAndNames, w)
		return

	} else if err != nil {
		output.Error(err, "Unknown error retreiving list of users.", w, r)
		return
	}

	return
}

//GET ONE USER'S DATA
func GetOne(w http.ResponseWriter, r *http.Request) {
	//get user id from form value
	userId := r.FormValue("userId")
	userIdInt, _ := strconv.ParseInt(userId, 10, 64)

	//get user data
	//looks in memcache and in datastore
	c := appengine.NewContext(r)
	data, err := Find(c, userIdInt)
	if err != nil {
		output.Error(err, "Cannot look up user data.", w, r)
		return
	}

	//return user data
	output.Success("findUser", data, w)
	return
}

//**********************************************************************
//DATASTORE KEYS

//CREATE COMPLETE KEY FOR USER
//get the full complete key from just the IntID of a key
func getUserKeyFromId(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, datastoreKind, "", id, nil)
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
	c := appengine.NewContext(r)
	q := datastore.NewQuery(datastoreKind).Filter("Username = ", adminUsername).KeysOnly()
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
func Find(c context.Context, userId int64) (User, error) {
	//memcache
	var r User
	userIdStr := strconv.FormatInt(userId, 10)
	_, err := memcache.Gob.Get(c, userIdStr, &r)

	if err == nil {
		//data found in memcache
		return r, nil
	} else if err == memcache.ErrCacheMiss {
		//data not found in memcache
		//look in datastore
		key := getUserKeyFromId(c, userId)
		q := datastore.NewQuery(datastoreKind).Filter("__key__ =", key).Limit(1)
		result := make([]User, 0, 1)
		_, err := q.GetAll(c, &result)
		if err != nil {
			return User{}, err
		}

		//return error if no result exists
		if len(result) == 0 {
			return User{}, ErrUserDoesNotExist
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
func exists(c context.Context, username string) (int64, User, error) {
	q := datastore.NewQuery(datastoreKind).Filter("Username = ", username).Limit(1)
	result := make([]User, 0, 1)
	keys, _ := q.GetAll(c, &result)

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
