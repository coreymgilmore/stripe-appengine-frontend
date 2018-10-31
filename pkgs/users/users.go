/*
Package users implements functionality for adding users, editing a user, and logging in a user.
*/
package users

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"cloud.google.com/go/datastore"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/templates"
	"google.golang.org/api/iterator"
)

const (
	//adminUsername is the default "super admin" username
	//this is created the first time the app is run and no datastore data exists yet
	adminUsername = "administrator"

	//minPwdLength is the shortest a new password can be for security concerns
	//this seems like a "reasonable" minimum requirement in 2018
	minPwdLength = 10
)

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

//userList is used to return the list of users when building select elements in the gui
type userList struct {
	ID       int64  `json:"id"`       //the app engine datastore id of the user
	Username string `json:"username"` //email address
}

//GetAll retrieves the list of all users in the datastore
//This is used to populate the select elements in the gui when changing a user's password or access rights.
func GetAll(w http.ResponseWriter, r *http.Request) {
	//connect to datastore
	c := r.Context()
	client, err := datastoreutils.Connect(c)
	if err != nil {
		output.Error(err, "Could not connect to datastore", w)
		return
	}

	//get list from datastore
	//only need to get username and entity key to cut down on datastore usage
	list := []userList{}
	q := datastore.NewQuery(datastoreutils.EntityUsers).Order("Username").Project("Username")
	i := client.Run(c, q)
	for {
		one := User{}
		key, err := i.Next(&one)

		if err == iterator.Done {
			break
		}
		if err != nil {
			output.Error(err, "Error retrieving list of users from datastore.", w)
			return
		}

		l := userList{
			Username: one.Username,
			ID:       key.ID,
		}
		list = append(list, l)
	}

	//return data to clinet
	output.Success("userList", list, w)
	return
}

//GetOne retrieves the full data for one user
//This is used to fill in the edit user modal in the gui.
func GetOne(w http.ResponseWriter, r *http.Request) {
	//get input
	userIDInt, _ := strconv.ParseInt(r.FormValue("userId"), 10, 64)

	//get user data
	c := r.Context()
	data, err := Find(c, userIDInt)
	if err != nil {
		output.Error(err, "Could not look up user's data.", w)
		return
	}

	//return user data
	output.Success("findUser", data, w)
	return
}

//DoesAdminExist checks if the super-admin has already been created
//The super-admin should be created upon initially using and setting up the app.
//This user must exist for the app to function.
func DoesAdminExist(r *http.Request) error {
	//connect to datastore
	c := r.Context()
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return err
	}

	//query
	//using GetAll instead of Get because you cannot filter using Get
	var user User
	q := datastore.NewQuery(datastoreutils.EntityUsers).Filter("Username = ", adminUsername)
	i := client.Run(c, q)
	for {
		_, err = i.Next(&user)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
	}

	//check if data was found
	//if it was, user will not equal an empty struct
	if user != (User{}) {
		return nil
	}

	//admin user does not exist
	return ErrAdminDoesNotExist
}

//getDataByUsername looks up data about a user by the user's username
func getDataByUsername(c context.Context, username string) (keyID int64, u User, err error) {
	//connect to datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return
	}

	//query
	//using GetAll instead of Get because you cannot filter using Get
	q := datastore.NewQuery(datastoreutils.EntityUsers).Filter("Username = ", adminUsername).Limit(1)
	i := client.Run(c, q)
	var numResults int
	var fullKey *datastore.Key
	for {
		fullKey, err = i.Next(&u)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, u, err
		}

		numResults++
	}

	//check if no user was found matching this username
	if numResults == 0 {
		return 0, u, ErrUserDoesNotExist
	}

	//user found
	return fullKey.ID, u, nil
}

//Find gets the data for a given user id
//This returns all the info on a user.
func Find(c context.Context, userID int64) (u User, err error) {
	//connect to datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return
	}

	key := datastoreutils.GetKeyFromID(datastoreutils.EntityUsers, userID)

	//query
	err = client.Get(c, key, &u)
	return
}

//exists checks if a given username is already being used
//This can also be used to get user data by username.
//Returns error if a user by the username 'username' does not exist.
func exists(c context.Context, username string) (keyID int64, u User, err error) {
	//connect to datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return
	}

	//query
	//using GetAll instead of Get because you cannot filter using Get
	var user User
	var fullKey *datastore.Key
	q := datastore.NewQuery(datastoreutils.EntityUsers).Filter("Username = ", username)
	i := client.Run(c, q)
	for {
		fullKey, err = i.Next(&user)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return
		}
	}

	//check if data was found
	//if it wasn't, user will equal a blank struct
	if user == (User{}) {
		return 0, u, ErrUserDoesNotExist
	}

	//return user found data
	return fullKey.ID, user, nil
}

//notificationPage is used to show html page for errors
//same as pages.notificationPage but have to have separate function b/c of dependency circle
func notificationPage(w http.ResponseWriter, panelType, title string, err interface{}, btnType, btnPath, btnText string) {
	data := templates.NotificationPage{
		PanelColor: panelType,
		Title:      title,
		Message:    err,
		BtnColor:   btnType,
		LinkHref:   btnPath,
		BtnText:    btnText,
	}

	templates.Load(w, "notifications", data)
	return
}
