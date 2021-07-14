package users

import (
	"net/http"
	"strconv"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sqliteutils"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/pwds"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
)

//ChangePwd is used to change a user's password
func ChangePwd(w http.ResponseWriter, r *http.Request) {
	//get inputs
	userID := r.FormValue("userId")
	userIDInt, _ := strconv.ParseInt(userID, 10, 64)
	password1 := r.FormValue("pass1")
	password2 := r.FormValue("pass2")

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

	//get user data
	c := r.Context()
	userData, err := Find(c, userIDInt)
	if err != nil {
		output.Error(err, "Error while retreiving user data to update user's password.", w)
		return
	}

	//set new password
	userData.Password = hashedPwd

	//generate full datastore key for user
	fullKey := datastoreutils.GetKeyFromID(datastoreutils.EntityUsers, userIDInt)

	//save user
	if sqliteutils.Config.UseSQLite {
		err = updateUserSqlite(userIDInt, userData)
	} else {
		_, err = saveUserDatastore(c, fullKey, userData)
	}

	if err != nil {
		output.Error(err, "Error saving user to database after password change.", w)
		return
	}

	//done
	output.Success("userChangePassword", nil, w)
}

//UpdatePermissions is used to save changes to a user's permissions (access rights)
//Super-admin "administrator" account cannot be edited...this user always has full permissions.
//You can not edit your own permissions so you don't lock yourself out of the app.
//Permissions default to "false".
func UpdatePermissions(w http.ResponseWriter, r *http.Request) {
	//gather form values
	userID := r.FormValue("userId")
	userIDInt, _ := strconv.ParseInt(userID, 10, 64)
	addCards, _ := strconv.ParseBool(r.FormValue("addCards"))
	removeCards, _ := strconv.ParseBool(r.FormValue("removeCards"))
	chargeCards, _ := strconv.ParseBool(r.FormValue("chargeCards"))
	viewReports, _ := strconv.ParseBool(r.FormValue("reports"))
	isAdmin, _ := strconv.ParseBool(r.FormValue("admin"))
	isActive, _ := strconv.ParseBool(r.FormValue("active"))

	//check if the logged in user is an admin
	//user updating another user's permission must be an admin
	//failsafe/second check since non-admins would not see the settings panel anyway
	session := sessionutils.Get(r)
	if session.IsNew {
		output.Error(errSessionMismatch, "An error occured. Please log out and log back in.", w)
		return
	}

	//get user data to update
	c := r.Context()
	userData, err := Find(c, userIDInt)
	if err != nil {
		output.Error(err, "We could not retrieve this user's information. This user could not be updated.", w)
		return
	}

	//check if the logged in user is trying to update their own permissions
	//you cannot edit your own permissions no matter what
	if session.Values["username"].(string) == userData.Username {
		output.Error(errCannotUpdateSelf, "You cannot edit your own permissions. Please contact another administrator.", w)
		return
	}

	//check if user is editing the super admin user
	if userData.Username == adminUsername {
		output.Error(errCannotUpdateSuperAdmin, "You cannot update the 'administrator' user. The account is locked.", w)
		return
	}

	//update the user
	userData.AddCards = addCards
	userData.RemoveCards = removeCards
	userData.ChargeCards = chargeCards
	userData.ViewReports = viewReports
	userData.Administrator = isAdmin
	userData.Active = isActive

	//generate complete key for user
	fullKey := datastoreutils.GetKeyFromID(datastoreutils.EntityUsers, userIDInt)

	//save user
	if sqliteutils.Config.UseSQLite {
		err = updateUserSqlite(userIDInt, userData)
	} else {
		_, err = saveUserDatastore(c, fullKey, userData)
	}

	if err != nil {
		output.Error(err, "Error saving user to database after permissions change.", w)
		return
	}

	//done
	output.Success("userUpdatePermissins", nil, w)
}

//updateUserSqlite updates a user in the sqlite db
func updateUserSqlite(id int64, u User) error {
	c := sqliteutils.Connection
	q := `
		UPDATE ` + sqliteutils.TableUsers + ` SET
			Username = ?,
			Password = ?,
			AddCards = ?,
			RemoveCards = ?,
			ViewReports = ?,
			Administrator = ?,
			Active = ?
		WHERE ID = ?
	`
	stmt, err := c.Prepare(q)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(
		u.Username,
		u.Password,
		u.AddCards,
		u.RemoveCards,
		u.ViewReports,
		u.Administrator,
		u.Active,
		id,
	)
	return err
}
