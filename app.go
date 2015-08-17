package stripeappenginefrontent

import (
	"net/http"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"templates"
	"sessionutils"
	"pages"
	"card"
	"users"
	"middleware"
)

func init() {
	//**********************************************************************
	//INIT

	//BUILD TEMPLATES
	templates.Build()

	//INIT SESSIONS
	sessionutils.Init()

	//INIT STRIPE
	card.Init()

	//**********************************************************************
	//MIDDLEWARE
	auth := alice.New(middleware.Auth)


	//**********************************************************************
	//ROUTER
	r := mux.NewRouter()
	r.StrictSlash(true)
	
	//root & setup
	r.HandleFunc("/", 				pages.Root)
	r.HandleFunc("/setup/", 		pages.CreateAdminShow)
	r.HandleFunc("/create-admin/", 	users.CreateAdmin).Methods("POST")
	r.HandleFunc("/login/",			users.Login)
	r.HandleFunc("/logout/", 		users.Logout)

	//logged in
	main := http.HandlerFunc(pages.Main)
	r.Handle("/main/", auth.Then(main))
	


	










	c := r.PathPrefix("/card").Subrouter()
	c.HandleFunc("/add/", 				card.Add)


	//PAGES THAT DO NOT EXIST
	r.NotFoundHandler = http.HandlerFunc(pages.NotFound)

	//LISTEN
	http.Handle("/", r)
}
