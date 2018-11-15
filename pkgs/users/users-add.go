package users

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"cloud.google.com/go/datastore"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/appsettings"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/company"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/pwds"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sqliteutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/timestamps"
)

//CreateAdmin saves the initial super-admin for the app
//This user is used to log in and create new users when the app is first used.
//This user is created when the app upon initial use of the app.
//Done this way b/c we don't want to set a default password/username in the code.
func CreateAdmin(w http.ResponseWriter, r *http.Request) {
	//make sure the admin user doesnt already exist
	err := DoesAdminExist(r)
	if err == nil {
		notificationPage(w, "panel-danger", "Error", "The admin user already exists.", "btn-default", "/", "Go Back")
		return
	}

	//get inputs
	pass1 := r.FormValue("password1")
	pass2 := r.FormValue("password2")

	//make sure passwords match
	if pass1 != pass2 {
		notificationPage(w, "panel-danger", "Error", "The passwords did not match.", "btn-default", "/setup/", "Try Again")
		return
	}

	//make sure the password is long enough
	if len(pass1) < minPwdLength {
		notificationPage(w, "panel-danger", "Error", "The password you provided is too short. It must me at least "+strconv.FormatInt(minPwdLength, 10)+" characters.", "btn-default", "/setup/", "Try Again")
		return
	}

	//hash the password
	hashedPwd := pwds.Create(pass1)

	//gather data
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

	//save to correct database
	var newUserID int64
	var outerErr error
	if sqliteutils.Config.UseSQLite {
		newUserID, outerErr = addUserSqlite(u)

	} else {
		//save to datastore
		ctx := r.Context()
		incompleteKey := datastoreutils.GetNewIncompleteKey(datastoreutils.EntityUsers)
		completeKey, err := saveUserDatastore(ctx, incompleteKey, u)
		outerErr = err
		newUserID = completeKey.ID
	}

	if outerErr != nil {
		notificationPage(w, "panel-danger", "Error", "An error occured while saving the admin user. "+err.Error(), "btn-default", "/setup/", "Try Again")
		return
	}

	//save user to session
	session := sessionutils.Get(r)
	if session.IsNew == false {
		notificationPage(w, "panel-danger", "Error", "An error occured while saving the admin user. Please clear your cookies and restart your browser.", "btn-default", "/setup/", "Try Again")
		return
	}
	sessionutils.AddValue(session, "username", adminUsername)
	sessionutils.AddValue(session, "user_id", newUserID)
	sessionutils.Save(session, w, r)

	//save the default company info
	//ignore errors since app will still work without this data being set
	//and user will see errors about missing stuff in the app
	ctx := r.Context()
	err = company.SaveDefaultInfo(ctx)
	if err != nil {
		log.Println("users.CreateAdmin", "Could not save default company info.", err)
		return
	}

	//save the default app settings
	err = appsettings.SaveDefaultInfo(ctx)
	if err != nil {
		log.Println("users.CreateAdmin", "Could not save default app settings.", err)
		return
	}

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
	c := r.Context()
	_, _, err := getDataByUsername(c, username)
	if err == nil {
		//user already exists
		//notify client
		output.Error(errUserAlreadyExists, "This username already exists. Please choose a different username.", w)
		return
	}

	//make sure passwords match
	if password1 != password2 {
		output.Error(errPasswordsDoNotMatch, "The passwords you provided to not match.", w)
		return
	}

	//make sure password is long enough
	if len(password1) < minPwdLength {
		output.Error(errPasswordTooShort, "The password you provided is too short. It must be at least "+strconv.FormatInt(minPwdLength, 10)+" characters.", w)
		return
	}

	//hash the password
	hashedPwd := pwds.Create(password1)

	//gather data to save new user
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

	//use correct db
	if sqliteutils.Config.UseSQLite {
		_, err = addUserSqlite(u)
	} else {
		incompleteKey := datastoreutils.GetNewIncompleteKey(datastoreutils.EntityUsers)
		_, err = saveUserDatastore(c, incompleteKey, u)
	}

	if err != nil {
		log.Println("users.Add-could not save user", err)
		output.Error(err, "Could not save user.", w)
		return
	}

	//respond to client with success message
	output.Success("addNewUser", nil, w)
	return
}

//saveUserDatastore saves a new user to the datastore
func saveUserDatastore(c context.Context, key *datastore.Key, user User) (*datastore.Key, error) {
	//connect to datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return key, err
	}

	//save to datastore
	completeKey, err := client.Put(c, key, &user)
	if err != nil {
		return completeKey, err
	}

	//done
	return completeKey, nil
}

//addUserSqlite saves a new user to the sqlite db
func addUserSqlite(user User) (int64, error) {
	c := sqliteutils.Connection
	q := `
			INSERT INTO ` + sqliteutils.TableUsers + ` (
				Username,
				Password,
				AddCards,
				RemoveCards,
				ChargeCards,
				ViewReports,
				Administrator,
				Active,
				Created
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
	stmt, err := c.Prepare(q)
	if err != nil {

		return 0, err
	}

	res, err := stmt.Exec(
		user.Username,
		user.Password,
		user.AddCards,
		user.RemoveCards,
		user.ChargeCards,
		user.ViewReports,
		user.Administrator,
		user.Active,
		user.Created,
	)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	return id, err
}
