package stripeappenginefrontent

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/justinas/alice"

	"card"
	"middleware"
	"pages"
	"receipt"
	"sessionutils"
	"templates"
	"users"
)

func init() {
	//**********************************************************************
	//INIT

	//BUILD TEMPLATES
	templates.Init()

	//INIT SESSIONS
	sessionutils.Init()

	//INIT STRIPE
	card.Init()

	//INIT RECEIPTS
	receipt.Init()

	//**********************************************************************
	//MIDDLEWARE
	a := alice.New(middleware.Auth)
	admin := alice.New(middleware.Auth, middleware.Administrator)
	add := alice.New(middleware.Auth, middleware.AddCards)
	remove := alice.New(middleware.Auth, middleware.RemoveCards)
	charge := alice.New(middleware.Auth, middleware.ChargeCards)
	reports := alice.New(middleware.Auth, middleware.ViewReports)

	//**********************************************************************
	//ROUTER
	r := mux.NewRouter()
	r.StrictSlash(true)

	//root & setup
	r.HandleFunc("/", pages.Root)
	r.HandleFunc("/setup/", pages.CreateAdminShow)
	r.HandleFunc("/create-admin/", users.CreateAdmin).Methods("POST")
	r.HandleFunc("/login/", users.Login)
	r.HandleFunc("/logout/", users.Logout)

	//logged in
	main := http.HandlerFunc(pages.Main)
	r.Handle("/main/", a.Then(main))

	//handlers
	usersAdd := http.HandlerFunc(users.Add)
	usersGetOne := http.HandlerFunc(users.GetOne)
	usersGetAll := http.HandlerFunc(users.GetAll)
	usersChangePwd := http.HandlerFunc(users.ChangePwd)
	usersUpdate := http.HandlerFunc(users.UpdatePermissions)
	cardsAdd := http.HandlerFunc(card.Add)
	cardsGetOne := http.HandlerFunc(card.GetOne)
	cardsGetAll := http.HandlerFunc(card.GetAll)
	cardsRemove := http.HandlerFunc(card.Remove)
	cardsCharge := http.HandlerFunc(card.Charge)
	cardsReceipt := http.HandlerFunc(receipt.Show)
	cardsReports := http.HandlerFunc(card.Report)
	cardsRefund := http.HandlerFunc(card.Refund)

	//users
	u := r.PathPrefix("/users").Subrouter()
	u.Handle("/add/", admin.Then(usersAdd)).Methods("POST")
	u.Handle("/get/", a.Then(usersGetOne)).Methods("GET")
	u.Handle("/get/all/", admin.Then(usersGetAll)).Methods("GET")
	u.Handle("/change-pwd/", admin.Then(usersChangePwd)).Methods("POST")
	u.Handle("/update/", admin.Then(usersUpdate)).Methods("POST")

	//cards
	c := r.PathPrefix("/card").Subrouter()
	c.Handle("/add/", add.Then(cardsAdd)).Methods("POST")
	c.Handle("/get/", a.Then(cardsGetOne)).Methods("GET")
	c.Handle("/get/all/", a.Then(cardsGetAll)).Methods("GET")
	c.Handle("/remove/", remove.Then(cardsRemove)).Methods("POST")
	c.Handle("/charge/", charge.Then(cardsCharge)).Methods("POST")
	c.Handle("/receipt/", a.Then(cardsReceipt)).Methods("GET")
	c.Handle("/report/", reports.Then(cardsReports)).Methods("GET")
	c.Handle("/refund/", charge.Then(cardsRefund)).Methods("POST")

	//PAGES THAT DO NOT EXIST
	r.NotFoundHandler = http.HandlerFunc(pages.NotFound)

	//LISTEN
	http.Handle("/", r)
}
