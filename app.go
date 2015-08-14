package stripeappenginefrontent

import (
	"net/http"
	"fmt"

	"github.com/gorilla/mux"
	//"github.com/justinas/alice"


	"templates"
	"sessionutils"
	"pages"
	"card"
)

var (
	sessionError error
	stripeKeyError error
)

func init() {
	//**********************************************************************
	//INIT

	//BUILD TEMPLATES
	templates.Build()

	//INIT SESSIONS
	sessionError = sessionutils.Init()

	//INIT STRIPE
	stripeKeyError = card.Init()

	//**********************************************************************
	//MIDDLEWARE


	//**********************************************************************
	//ROUTER
	r := mux.NewRouter()
	r.StrictSlash(true)
	
	//general pages
	r.HandleFunc("/", 				pages.Root)
	r.HandleFunc("/init-errors/", 	checkInitErrors)
	r.HandleFunc("/setup/", 		pages.CreateAdminShow)
	r.HandleFunc("/create-admin/", 	pages.CreateAdminDo).Methods("POST")


	//logged in pages
	r.HandleFunc("/main/", 	pages.Main)
	
	c := r.PathPrefix("/card").Subrouter()
	c.HandleFunc("/add/", 				card.Add)


	//PAGES THAT DO NOT EXIST
	r.NotFoundHandler = http.HandlerFunc(pages.NotFound)

	//LISTEN
	http.Handle("/", r)
}

//SHOW ERRORS READING NECESSARY SECRET FILES
//displays <nil> if the files were found and read into the app
//if the files cannot be found, an error is provided
func checkInitErrors(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Session Init Errors\n")
	fmt.Fprint(w, sessionError)
	fmt.Fprint(w, "\n\n")
	fmt.Fprint(w, "Stripe Key Error\n")
	fmt.Fprint(w, stripeKeyError)
	return
}
