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

//userList is used to return the list of users able to be edited to build the gui
//This list is used to build select menus in the gui.
type userList struct {
	ID       int64  `json:"id"`       //the app engine datastore id of the user
	Username string `json:"username"` //email address
}

//GetAll retrieves the list of all users in the datastore
//The data is pulled from the datastore.
//The data is returned as a json to populate select menus in the gui.
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
	q := datastore.NewQuery(datastoreutils.EntityUsers).Order("Username").Project("Username")
	var users []User
	keys, err := client.GetAll(c, q, &users)
	if err != nil {
		output.Error(err, "Error retrieving list of users from datastore.", w)
		return
	}

	//build result
	//format data to show just datastore id and username
	//creates a map of structs
	var idsAndNames []userList
	for i, r := range users {
		x := userList{
			Username: r.Username,
			ID:       keys[i].ID,
		}

		idsAndNames = append(idsAndNames, x)
	}

	//return data to clinet
	output.Success("userList", idsAndNames, w)

	return
}

//GetOne retrieves the full data for one user
//This is used to fill in the edit user modal in the gui.
func GetOne(w http.ResponseWriter, r *http.Request) {
	//get user id from form value
	userIDInt, _ := strconv.ParseInt(r.FormValue("userId"), 10, 64)

	//get user data
	c := r.Context()
	data, err := Find(c, userIDInt)
	if err != nil {
		output.Error(err, "Cannot look up user data.", w)
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

	var user []User
	q := datastore.NewQuery(datastoreutils.EntityUsers).Filter("Username = ", adminUsername).KeysOnly()
	keys, err := client.GetAll(c, q, &user)
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

//Find gets the data for a given user id
//This returns all the info on a user.
func Find(c context.Context, userID int64) (u User, err error) {
	//connect to datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return
	}

	var uu []User
	key := datastoreutils.GetKeyFromID(datastoreutils.EntityUsers, userID)
	q := datastore.NewQuery(datastoreutils.EntityUsers).Filter("__key__ =", key).Limit(1)
	_, err = client.GetAll(c, q, &uu)

	//get one and only result
	if len(uu) > 0 {
		u = uu[0]
	}

	return
}

//exists checks if a given username is already being used
//This can also be used to get user data by username.
//Returns error if a user by the username 'username' does not exist.
func exists(c context.Context, username string) (int64, User, error) {
	//connect to datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return 0, User{}, ErrUserDoesNotExist
	}

	q := datastore.NewQuery(datastoreutils.EntityUsers).Filter("Username = ", username).Limit(1)
	var result []User
	keys, _ := client.GetAll(c, q, &result)

	//user was not found
	if len(keys) == 0 {
		return 0, User{}, ErrUserDoesNotExist
	}

	//return user found data
	return keys[0].ID, result[0], nil
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
