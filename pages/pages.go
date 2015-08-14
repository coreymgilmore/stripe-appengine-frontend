package pages 

import (
	"net/http"

	"templates"
)

//STRUCT FOR HOLDING TEMPLATE DATA
type tempData struct {
	PanelColor 		string
	Title 			string
	Message 		string
	BtnColor 		string
	LinkHref 		string
	BtnText 		string
}

//MAIN ROOT PAGES
func Root(w http.ResponseWriter, r *http.Request) {
	templates.Load(w, "root", nil)
	return
}

//PAGES THAT DO NOT EXIST
func NotFound(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("This page does not exist and cannot be found. :("))
	return
}

//MAIN LOGGED IN PAGE
func Main(w http.ResponseWriter, r *http.Request) {
	templates.Load(w, "main", nil)
	return
}

//LOAD THE PAGE TO CREATE THE INITIAL ADMIN USER
func CreateAdminShow(w http.ResponseWriter, r *http.Request) {
	templates.Load(w, "create-admin", nil)
	return
}

//SAVE THE INITIAL ADMIN USER
func CreateAdminDo(w http.ResponseWriter, r *http.Request) {
	//get form values
	pass1 := r.FormValue("password1")
	pass2 := r.FormValue("password2")

	//make sure they match
	if pass1 != pass2 {
		templates.Load(w, "create-admin-notifications", tempData{"panel-danger", "Error", "The passwords you provided did not match.", "btn-default", "/setup/", "Try Again"})
		return
	}

	//make sure the password is long enough
	if len(pass1) < 8 {
		templates.Load(w, "create-admin-notifications", tempData{"panel-danger", "Error", "The password you provided is not long enough.", "btn-default", "/setup/", "Try Again"})
		return
	}

	//save the user
	_ = "admin@example.com"
	return

}