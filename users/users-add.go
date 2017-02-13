/*
File users-add.go adds new users to the app.
*/

package users

import (
	"golang.org/x/net/context"
	"fmt"
	"memcacheutils"
	"net/http"
	"output"
	"pwds"
	"sessionutils"
	"strconv"
	"timestamps"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

//CreateAdmin saves the initial super-admin for the app
//this user is used to log in and create new users
//this user is created when the app is first deployed and used
//done this way b/c we don't want to set a default password/username in the code
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
	if len(pass1) < minPwdLength {
		notificationPage(w, "panel-danger", "Error", "The password you provided is too short. It must me at least "+strconv.FormatInt(minPwdLength, 10)+" characters.", "btn-default", "/setup/", "Try Again")
		return
	}

	//hash the password
	hashedPwd := pwds.Create(pass1)

	//create the user
	u := User{
		Username:      adminUsername,
		Password:      hashedPwd,
		AddCards:      true,
		RemoveCards:   true,
		ChargeCards:   true,
		ViewReports:   true,
		Administrator: true,
		Active:        true,
		Created:       timestamps.ISO8601(),
	}

	//save to datastore
	c := appengine.NewContext(r)
	incompleteKey := createNewUserKey(c)
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
	sessionutils.AddValue(session, "username", adminUsername)
	sessionutils.AddValue(session, "user_id", completeKey.IntID())
	sessionutils.Save(session, w, r)

	//show user main page
	http.Redirect(w, r, "/main/", http.StatusFound)
	return
}

//Add saves a new user to the app
func Add(w http.ResponseWriter, r *http.Request) {
	//get form values
	username := r.FormValue("username")
	password1 := r.FormValue("password1")
	password2 := r.FormValue("password2")
	addCards, _ := strconv.ParseBool(r.FormValue("addCards"))
	removeCards, _ := strconv.ParseBool(r.FormValue("removeCards"))
	chargeCards, _ := strconv.ParseBool(r.FormValue("chargeCards"))
	viewReports, _ := strconv.ParseBool(r.FormValue("reports"))
	isAdmin, _ := strconv.ParseBool(r.FormValue("admin"))
	isActive, _ := strconv.ParseBool(r.FormValue("active"))

	//check if this user already exists
	c := appengine.NewContext(r)
	_, _, err := exists(c, username)
	if err == nil {
		//user already exists
		//notify client
		output.Error(ErrUserAlreadyExists, "This username already exists. Please choose a different username.", w, r)
		return
	}

	//make sure passwords match
	if doStringsMatch(password1, password2) == false {
		output.Error(ErrPasswordsDoNotMatch, "The passwords you provided to not match.", w, r)
		return
	}

	//make sure password is long enough
	if len(password1) < minPwdLength {
		output.Error(ErrPasswordTooShort, "The password you provided is too short. It must be at least "+strconv.FormatInt(minPwdLength, 10)+" characters.", w, r)
		return
	}

	//hash the password
	hashedPwd := pwds.Create(password1)

	//create the user
	u := User{
		Username:      username,
		Password:      hashedPwd,
		AddCards:      addCards,
		RemoveCards:   removeCards,
		ChargeCards:   chargeCards,
		ViewReports:   viewReports,
		Administrator: isAdmin,
		Active:        isActive,
		Created:       timestamps.ISO8601(),
	}

	//save to datastore
	incompleteKey := createNewUserKey(c)
	_, err = saveUser(c, incompleteKey, u)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	//clear list of users saved in memcache since a new user was added
	memcacheutils.Delete(c, listOfUsersKey)

	//respond to client with success message
	output.Success("addNewUser", nil, w)
	return
}

//createNewCustomerKey generates a new datastore key for saving a new user
//appengine's datastore does not generate this key automatically when an entity is saved
func createNewUserKey(c context.Context) *datastore.Key {
	return datastore.NewIncompleteKey(c, datastoreKind, nil)
}

//saveUser does the actual saving of a user to the datastore
//separate function to clean up code
func saveUser(c context.Context, key *datastore.Key, user User) (*datastore.Key, error) {
	//save to datastore
	completeKey, err := datastore.Put(c, key, &user)
	if err != nil {
		return completeKey, err
	}

	//save user to memcache
	mKey := strconv.FormatInt(completeKey.IntID(), 10)
	err = memcacheutils.Save(c, mKey, user)
	if err != nil {
		return completeKey, err
	}

	//done
	return completeKey, nil
}
