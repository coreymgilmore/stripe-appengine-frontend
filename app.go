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

	//users
	u := r.PathPrefix("/users").Subrouter()
	usersAdd := 		http.HandlerFunc(users.Add)
	usersGetOne := 		http.HandlerFunc(users.GetOne)
	usersGetAll := 		http.HandlerFunc(users.GetAll)
	usersChangePwd := 	http.HandlerFunc(users.ChangePwd)

	u.Handle("/add/", 			auth.Then(usersAdd))
	u.Handle("/get/", 			auth.Then(usersGetOne))
	u.Handle("/get/all/", 		auth.Then(usersGetAll))
	u.Handle("/change-pwd/", 	auth.Then(usersChangePwd))


	










	c := r.PathPrefix("/card").Subrouter()
	c.HandleFunc("/add/", 				card.Add)


	//PAGES THAT DO NOT EXIST
	r.NotFoundHandler = http.HandlerFunc(pages.NotFound)

	//LISTEN
	http.Handle("/", r)
}
