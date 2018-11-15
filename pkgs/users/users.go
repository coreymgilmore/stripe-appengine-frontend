/*
Package users implements functionality for adding users, editing a user, and logging in a user.
*/
package users

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sqliteutils"

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

	//fields not used in cloud datastore
	ID int64 `json:"sqlite_user_id"`
}

//userList is used to return the list of users when building select elements in the gui
type userList struct {
	ID       int64  `json:"id"`       //the app engine datastore id of the user
	Username string `json:"username"` //email address
}

//GetAll retrieves the list of all users in the datastore
//This is used to populate the select elements in the gui when changing a user's password or access rights.
func GetAll(w http.ResponseWriter, r *http.Request) {
	//placeholder
	list := []userList{}

	//use correct db
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		q := `
			SELECT ID, Username
			FROM ` + sqliteutils.TableUsers
		err := c.Select(&list, q)
		if err != nil {
			output.Error(err, "Error retreiving list of users from sqlite.", w)
			return
		}

	} else {
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
	//query
	c := r.Context()
	_, _, err := getDataByUsername(c, adminUsername)
	if err == nil {
		return nil
	} else if err == ErrUserDoesNotExist {
		return ErrAdminDoesNotExist
	} else {
		return err
	}
}

//getDataByUsername looks up data about a user by the user's username
func getDataByUsername(c context.Context, username string) (int64, User, error) {
	//placeholder
	u := User{}

	//use correct db
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		q := `
			SELECT *
			FROM ` + sqliteutils.TableUsers + `
			WHERE Username = ?
		`
		err := c.Get(&u, q, username)
		if err == sql.ErrNoRows {
			return 0, u, ErrUserDoesNotExist
		} else if err != nil {
			return 0, u, err
		}
	} else {
		//connect to datastore
		client, err := datastoreutils.Connect(c)
		if err != nil {
			return 0, u, err
		}

		//query
		//using GetAll instead of Get because you cannot filter using Get
		q := datastore.NewQuery(datastoreutils.EntityUsers).Filter("Username = ", username).Limit(1)
		i := client.Run(c, q)
		var numResults int
		var fullKey *datastore.Key
		for {
			var tempUserData User
			tempKey, err := i.Next(&tempUserData)
			if err == iterator.Done {
				break
			}
			if err != nil {
				return 0, u, err
			}

			//save key and data to variables outside iterator
			fullKey = tempKey
			u = tempUserData
			numResults++
		}

		//check if no user was found matching this username
		if numResults == 0 {
			return 0, u, ErrUserDoesNotExist
		}

		//user found
		u.ID = fullKey.ID
	}

	//done
	return u.ID, u, nil
}

//Find gets the data for a given user id
//This returns all the info on a user.
func Find(c context.Context, userID int64) (User, error) {
	//placeholder
	u := User{}
	var err error

	//use correct db
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		q := `
			SELECT * 
			FROM ` + sqliteutils.TableUsers + `
			WHERE ID = ?
		`
		err = c.Get(&u, q, userID)
	} else {
		//connect to datastore
		client, err := datastoreutils.Connect(c)
		if err != nil {
			return u, err
		}

		//query
		key := datastoreutils.GetKeyFromID(datastoreutils.EntityUsers, userID)
		err = client.Get(c, key, &u)
	}

	return u, err
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
