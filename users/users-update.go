/*
	This is part of the users package.
	This deals with changing a user's password or updating a user's permissions.
	These functions are in a separate file for organization.
*/

package users

import (
	"net/http"
	"strconv"

	"google.golang.org/appengine"

	"github.com/coreymgilmore/pwds"

	"memcacheutils"
	"output"
	"sessionutils"
)

//UPDATE A USER'S PASSWORD
func ChangePwd(w http.ResponseWriter, r *http.Request) {
	//gather inputs
	userId := r.FormValue("userId")
	userIdInt, _ := strconv.ParseInt(userId, 10, 64)
	password1 := r.FormValue("pass1")
	password2 := r.FormValue("pass2")

	//make sure passwords match
	if doStringsMatch(password1, password2) == false {
		output.Error(ErrPasswordsDoNotMatch, "The passwords you provided to not match.", w)
		return
	}

	//make sure password is long enough
	if len(password1) < MIN_PASSWORD_LENGTH {
		output.Error(ErrPasswordTooShort, "The password you provided is too short. It must be at least "+strconv.FormatInt(MIN_PASSWORD_LENGTH, 10)+" characters.", w)
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
	err = memcacheutils.Delete(c, userId)
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
//super-admin "administrator" account cannot be edited...this user always has full permissions
//you can also not edit your own permissions...so you don't lock yourself out of the app
func UpdatePermissions(w http.ResponseWriter, r *http.Request) {
	//gather form values
	userId := r.FormValue("userId")
	userIdInt, _ := strconv.ParseInt(userId, 10, 64)
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
		output.Error(ErrSessionMismatch, "An error occured. Please log out and log back in.", w)
		return
	}

	//get user data to update
	c := appengine.NewContext(r)
	userData, err := Find(c, userIdInt)
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
	userData.AddCards = addCards
	userData.RemoveCards = removeCards
	userData.ChargeCards = chargeCards
	userData.ViewReports = viewReports
	userData.Administrator = isAdmin
	userData.Active = isActive

	//clear memcache
	err = memcacheutils.Delete(c, userId)
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
