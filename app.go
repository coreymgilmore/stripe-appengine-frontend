package stripeappenginefrontent

import (
	"net/http"
	
	"github.com/justinas/alice"
	"github.com/gorilla/mux"
	
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

	//users
	u := 				r.PathPrefix("/users").Subrouter()
	usersAdd := 		http.HandlerFunc(users.Add)
	usersGetOne := 		http.HandlerFunc(users.GetOne)
	usersGetAll := 		http.HandlerFunc(users.GetAll)
	usersChangePwd := 	http.HandlerFunc(users.ChangePwd)
	usersUpdate := 		http.HandlerFunc(users.UpdatePermissions)
	u.Handle("/add/", 			auth.Then(usersAdd)).Methods("POST")
	u.Handle("/get/", 			auth.Then(usersGetOne)).Methods("GET")
	u.Handle("/get/all/", 		auth.Then(usersGetAll)).Methods("GET")
	u.Handle("/change-pwd/", 	auth.Then(usersChangePwd)).Methods("POST")
	u.Handle("/update/", 		auth.Then(usersUpdate)).Methods("POST")

	//cards
	c := 				r.PathPrefix("/card").Subrouter()
	cardsAdd := 		http.HandlerFunc(card.Add)
	cardsGetOne := 		http.HandlerFunc(card.GetOne)
	cardsGetAll := 		http.HandlerFunc(card.GetAll)
	cardsRemove := 		http.HandlerFunc(card.Remove)
	cardsCharge := 		http.HandlerFunc(card.Charge)
	c.Handle("/add/", 				auth.Then(cardsAdd)).Methods("POST")
	c.Handle("/get/", 				auth.Then(cardsGetOne)).Methods("GET")
	c.Handle("/get/all/", 			auth.Then(cardsGetAll)).Methods("GET")
	c.Handle("/remove/", 			auth.Then(cardsRemove)).Methods("POST")
	c.Handle("/charge/", 			auth.Then(cardsCharge)).Methods("POST")

	//PAGES THAT DO NOT EXIST
	r.NotFoundHandler = http.HandlerFunc(pages.NotFound)

	//LISTEN
	http.Handle("/", r)
}
