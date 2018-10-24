/*
Package users implements functionality for adding users, editing a user, and logging in a user.
*/
package users

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/memcacheutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/templates"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
)

//datastoreKind is the appengine datastore "kind" is similar to a "collection" or "table" in other dbs.
const datastoreKind = "users"

//adminUsername is the default "super admin" username
//this is created the first time the app is run and no datastore data exists yet
const adminUsername = "administrator"

//minPwdLength is the shortest a new password can be for security concerns
//this seems like a "reasonable" minimum requirement in 2018
const minPwdLength = 10

//listOfUsersKey is the key name for storing the list of users in memcache
//it is used to get the list of users faster then having to query the datastore every time
const listOfUsersKey = "list-of-users"

//errors
var (
	ErrAdminDoesNotExist      = errors.New("users: admin user does not exist")
	ErrUserDoesNotExist       = errors.New("users: user does not exist")
	errUserAlreadyExists      = errors.New("users: user already exists")
	errPasswordsDoNotMatch    = errors.New("users: passwords do not match")
	errPasswordTooShort       = errors.New("users: password too short")
	errNotAdmin               = errors.New("users: user is not an admin")
	errSessionMismatch        = errors.New("users: session mismatch")
	errCannotUpdateSelf       = errors.New("users: cannot update yourself")
	errCannotUpdateSuperAdmin = errors.New("users: cannot update super admin")
)

//User is the format for data saved to the datastore about a user
type User struct {
	Username      string `json:"username"`         //an email address (exception is for the super-admin created initially)
	Password      string `json:"-"`                //bcrypt encrypted password
	AddCards      bool   `json:"add_cards"`        //permissions
	RemoveCards   bool   `json:"remove_cards"`     //" "
	ChargeCards   bool   `json:"charge_cards"`     //" "
	ViewReports   bool   `json:"view_reports"`     //" "
	Administrator bool   `json:"is_admin"`         //" "
	Active        bool   `json:"is_active"`        //is the user able to access the app
	Created       string `json:"datetime_created"` //datetime of when the user was created
}

//userList is used to return the list of users able to be edited to build the gui
//This list is used to build select menus in the gui.
type userList struct {
	ID       int64  `json:"id"`       //the app engine datastore id of the user
	Username string `json:"username"` //email address
}

//GetAll retrieves the list of all users in the datastore
//The data is pulled from memcache or the datastore.
//The data is returned as a json to populate select menus in the gui.
func GetAll(w http.ResponseWriter, r *http.Request) {
	c := r.Context(r)

	//check if list of users is in memcache
	var result []userList
	_, err := memcache.Gob.Get(c, listOfUsersKey, &result)
	if err == nil {
		output.Success("userList-cached", result, w)
		return
	}

	//list of users not found in memcache
	//get list from datastore
	//only need to get username and entity key to cut down on datastore usage
	//save the list to memcache for faster retrieval next time
	if err == memcache.ErrCacheMiss {
		q := datastore.NewQuery(datastoreKind).Order("Username").Project("Username")
		var users []User
		keys, err := q.GetAll(c, &users)
		if err != nil {
			output.Error(err, "Error retrieving list of users from datastore.", w, r)
			return
		}

		//build result
		//format data to show just datastore id and username
		//creates a map of structs
		var idsAndNames []userList
		for i, r := range users {
			x := userList{
				Username: r.Username,
				ID:       keys[i].IntID(),
			}

			idsAndNames = append(idsAndNames, x)
		}

		//save the list of users to memcache
		//ignore errors since we still retrieved the data
		memcacheutils.Save(c, listOfUsersKey, idsAndNames)

		//return data to clinet
		output.Success("userList", idsAndNames, w)

	} else if err != nil {
		output.Error(err, "Unknown error retrieving list of users.", w, r)
	}

	return
}

//GetOne retrieves the full data for one user
//This is used to fill in the edit user modal in the gui.
func GetOne(w http.ResponseWriter, r *http.Request) {
	//get user id from form value
	userIDInt, _ := strconv.ParseInt(r.FormValue("userId"), 10, 64)

	//get user data
	c := r.Context(r)
	data, err := Find(c, userIDInt)
	if err != nil {
		output.Error(err, "Cannot look up user data.", w, r)
		return
	}

	//return user data
	output.Success("findUser", data, w)
	return
}

//getUserKeyFromID gets the full datastore key from the id
//ID is just numeric, key is a big string with the appengine name, kind name, etc.
//Key is what is actually used to find entities in the datastore.
func getUserKeyFromID(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, datastoreKind, "", id, nil)
}

//DoesAdminExist checks if the super-admin has already been created
//The super-admin should be created upon initially using and setting up the app.
//This user must exist for the app to function.
func DoesAdminExist(r *http.Request) error {
	//query for admin user
	var user []User
	c := r.Context(r)
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

//AllowedAccess checks if a given user is allowed access to the app
//this checks the user's permissions
func AllowedAccess(data User) bool {
	return data.Active
}

//doStringsMatch checks if two given input strings are the same
//This is used when checking the two given passwords for a new user.
//Just cleans up code elsewhere.
func doStringsMatch(string1, string2 string) bool {
	if string1 == string2 {
		return true
	}

	return false
}

//Find gets the data for a given user id
//This returns all the info on a user.
//First memcache is checked for the data, then the datastore.
func Find(c context.Context, userID int64) (u User, err error) {
	//check for card in memcache
	userIDStr := strconv.FormatInt(userID, 10)
	_, err = memcache.Gob.Get(c, userIDStr, &u)
	if err == nil {
		return
	}

	//user data not found in memcache
	//look up data in datastore
	//save to memcache after it is found
	if err == memcache.ErrCacheMiss {
		var uu []User
		key := getUserKeyFromID(c, userID)
		q := datastore.NewQuery(datastoreKind).Filter("__key__ =", key).Limit(1)
		_, err = q.GetAll(c, &uu)

		//get one and only result
		if len(uu) > 0 {
			u = uu[0]
		}

		//save to memcache
		//ignore errors since we already got the data
		memcacheutils.Save(c, userIDStr, u)
	}

	return
}

//exists checks if a given username is already being used
//This can also be used to get user data by username.
//Returns error if a user by the username 'username' does not exist.
func exists(c context.Context, username string) (int64, User, error) {
	q := datastore.NewQuery(datastoreKind).Filter("Username = ", username).Limit(1)
	var result []User
	keys, _ := q.GetAll(c, &result)

	//user was not found
	if len(keys) == 0 {
		return 0, User{}, ErrUserDoesNotExist
	}

	//return user found data
	return keys[0].IntID(), result[0], nil
}

//notificationPage is used to show html page for errors
//same as pages.notificationPage but have to have separate function b/c of dependency circle
func notificationPage(w http.ResponseWriter, panelType, title string, err interface{}, btnType, btnPath, btnText string) {
	templates.Load(w, "notifications", templates.NotificationPage{panelType, title, err, btnType, btnPath, btnText})
	return
}
