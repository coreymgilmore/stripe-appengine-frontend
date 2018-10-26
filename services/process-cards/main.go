/*
Package main implements a simple web app for collecting and charging credit cards.

This app was designed for use in companies that collect their clients cards and process
charges without the client needing to provide card information with every purchase.
Companies that use this app don't have to store credit card information, or ask for it with
every purchase to reduce security issues or remove need for PCI compliance.

This app was designed to run on Google App Engine and will not work in a normal golang
environment (as of Oct 2018).  This app was designed for the  Appengine Standard Environment
and the go111 runtime.  You must have a Google Cloud account.

This app uses Stripe as the payment processor.  You must have a Stripe account.

The only data stored for this app is user credentials, basic company information,
and a list of customers for which charges are processed against.  There is no need to store
the credit card's information (number, expiration, security code).  The card's information
is saved to Stripe and only an identifier is saved to this app. This id is used to process
the card with Stripe.

Payment are processed by either manually by choosing the customer and entering the payment
amount or via an api-style http request.
*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/appsettings"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/card"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/company"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/cron"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/middleware"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/pages"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/receipt"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	yaml "gopkg.in/yaml.v2"
)

//defaultPort is the port used when serving in the dev environment
const defaultPort = "8005"

const (
	//staticWebDir is the directory off of the root domain from which static files are served
	//www.example.com/static/ -? /static
	//leave off trailing slash
	staticWebDir = "/static"

	//staticLocalDir is the location on the server's filesystem where we store static files
	staticLocalDir = "./website/static/"
)

//cacheDays is the number of days to cache static files
//this is set in init() and used when serving static files
var cacheDays = 0

//appYaml is the format of the app.yaml file
type appYaml struct {
	Runtime string `yaml:"runtime"` //should be go111 for deployments to appengine
	EnvVars struct {
		ProjectID            string `yaml:"PROJECT_ID"`             //the project id on google cloud
		SessionAuthKey       string `yaml:"SESSION_AUTH_KEY"`       //session cookie
		SessionEncryptKey    string `yaml:"SESSION_ENCRYPT_KEY"`    //session cookie
		StripeSecretKey      string `yaml:"STRIPE_SECRET_KEY"`      //used for charging cards
		StripePublishableKey string `yaml:"STRIPE_PUBLISHABLE_KEY"` //used for creating customers and saving cards
		CacheDays            int    `yaml:"CACHE_DAYS"`             //number of days to cache static files
	} `yaml:"env_variables"`
	Handlers []struct {
		URL       string `yaml:"url"`
		StaticDir string `yaml:"static_dir"`
		Upload    string `yaml:"upload"`
		Script    string `yaml:"script"`
	} `yaml:"handlers"`
	ErrorHandlers []struct {
		File      string `yaml:"file"`
		ErrorCode string `yaml:"error_code"`
	} `yaml:"error_handlers"`
}

func init() {
	//use flags to allow for different deployment types
	//for example, developing locally or deployed on appengine
	//appengine is the default deployment
	//need to do this so we can parse app.yaml in a development environment so we don't need to specify each env_var manually on PATH
	//this also allows us in the future to create other deployment types such as a sqlite backed version instead of using cloud datastore
	deploymentType := flag.String("type", "appengine", "Set to appengine or appengine-dev.  In development mode the app.yaml file will be parsed to read the set environmental variables.")
	pathToAppYaml := flag.String("pathToAppYaml", "./app.yaml", "The path to the app.yaml file.")
	pathToDatastoreCredentials := flag.String("datastore-credentials", "./datastore-service-account.json", "The path to your datastore service account file.  A JSON file.")
	flag.Parse()

	//set configuration options based on deployment type
	switch *deploymentType {
	case "appengine":
		//the default deployment type
		//when this app is run on appengine, the environmental variables in app.yaml file will automatically be provided to the app
		c := sessionutils.Config
		c.SessionAuthKey = os.Getenv("SESSION_AUTH_KEY")
		c.SessionEncryptKey = os.Getenv("SESSION_ENCRYPT_KEY")
		err := sessionutils.SetConfig(c)
		if err != nil {
			log.Fatalln("Could not set configuration for sessionutils.", err)
			return
		}

		cc := card.Config
		cc.StripeSecretKey = os.Getenv("STRIPE_SECRET_KEY")
		cc.StripePublishableKey = os.Getenv("STRIPE_PUBLISHABLE_KEY")
		err = card.SetConfig(cc)
		if err != nil {
			log.Fatalln("Could not set configuration for card.", err)
			return
		}

		ccc := datastoreutils.Config
		ccc.ProjectID = os.Getenv("PROJECT_ID")
		err = datastoreutils.SetConfig(ccc)
		if err != nil {
			log.Fatalln("Could not set configuration for datastore.", err)
			return
		}

		//set cache max age
		cacheDays, _ = strconv.Atoi(os.Getenv("CACHE_DAYS"))

	case "appengine-dev":
		//check for and parse the app.yaml file
		yamlData, err := parseAppYaml(*pathToAppYaml)
		if err != nil {
			log.Fatalln("Error while parsing app.yaml.", err)
			return
		}

		//set configuration options using app.yaml
		//this is how we set the configuration in development
		c := sessionutils.Config
		c.SessionAuthKey = yamlData.EnvVars.SessionAuthKey
		c.SessionEncryptKey = yamlData.EnvVars.SessionEncryptKey
		err = sessionutils.SetConfig(c)
		if err != nil {
			log.Fatalln("Could not set configuration for sessionutils.", err)
			return
		}

		cc := card.Config
		cc.StripeSecretKey = yamlData.EnvVars.StripeSecretKey
		cc.StripePublishableKey = yamlData.EnvVars.StripePublishableKey
		err = card.SetConfig(cc)
		if err != nil {
			log.Fatalln("Could not set configuration for card.", err)
			return
		}

		//connect to the google cloud datastore
		//need to set an environmental variable here since it provide credentials to the datastore, see https://cloud.google.com/datastore/docs/reference/libraries#client-libraries-install-go
		err = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", *pathToDatastoreCredentials)
		if err != nil {
			log.Fatalln("Could not set Google Datastore credentials environmental variable.", err)
			return
		}

		//by default, this will connect to the Cloud datastore.
		//if you want to use a local development datastore, see https://cloud.google.com/datastore/docs/tools/datastore-emulator
		ccc := datastoreutils.Config
		ccc.ProjectID = yamlData.EnvVars.ProjectID
		err = datastoreutils.SetConfig(ccc)
		if err != nil {
			log.Fatalln("Could not set configuration for datastore.", err)
			return
		}

		//set cache max age
		cacheDays = yamlData.EnvVars.CacheDays

	case "sqlite":
		//a version of this app that can run without appengine and is backed by sqlite

	default:
		//when an invalid deployment type is given
		log.Fatalln("An invalid deployment type was given as a flag.")
		return
	}

	return
}

func main() {
	//middleware
	a := alice.New(middleware.Auth)
	admin := a.Append(middleware.Administrator)
	add := a.Append(middleware.AddCards)
	remove := a.Append(middleware.RemoveCards)
	charge := a.Append(middleware.ChargeCards)
	reports := a.Append(middleware.ViewReports)

	//router
	r := mux.NewRouter()
	r.StrictSlash(true)

	//basic pages
	r.HandleFunc("/", pages.Login)
	r.HandleFunc("/setup/", pages.CreateAdminShow)
	r.HandleFunc("/create-admin/", users.CreateAdmin).Methods("POST")
	r.HandleFunc("/login/", users.Login)
	r.HandleFunc("/logout/", users.Logout)

	//cron tasks
	r.HandleFunc("/cron/remove-expired-cards/", cron.RemoveExpiredCards)

	//main app page once user is logged in
	main := http.HandlerFunc(pages.Main)
	r.Handle("/main/", a.Then(main))

	//API endpoints
	//users
	u := r.PathPrefix("/users").Subrouter()
	u.Handle("/add/", admin.Then(http.HandlerFunc(users.Add))).Methods("POST")
	u.Handle("/get/", a.Then(http.HandlerFunc(users.GetOne))).Methods("GET")
	u.Handle("/get/all/", admin.Then(http.HandlerFunc(users.GetAll))).Methods("GET")
	u.Handle("/change-pwd/", admin.Then(http.HandlerFunc(users.ChangePwd))).Methods("POST")
	u.Handle("/update/", admin.Then(http.HandlerFunc(users.UpdatePermissions))).Methods("POST")

	//cards
	c := r.PathPrefix("/card").Subrouter()
	c.Handle("/add/", add.Then(http.HandlerFunc(card.Add))).Methods("POST")
	c.Handle("/get/", a.Then(http.HandlerFunc(card.GetOne))).Methods("GET")
	c.Handle("/get/all/", a.Then(http.HandlerFunc(card.GetAll))).Methods("GET")
	c.Handle("/remove/", remove.Then(http.HandlerFunc(card.RemoveAPI))).Methods("POST")
	c.Handle("/charge/", charge.Then(http.HandlerFunc(card.ManualCharge))).Methods("POST")
	c.Handle("/receipt/", a.Then(http.HandlerFunc(receipt.Show))).Methods("GET")
	c.Handle("/report/", reports.Then(http.HandlerFunc(card.Report))).Methods("GET")
	c.Handle("/refund/", charge.Then(http.HandlerFunc(card.Refund))).Methods("POST")

	c.Handle("/auto-charge/", http.HandlerFunc(card.AutoCharge)).Methods("POST")

	//company info
	comp := r.PathPrefix("/company").Subrouter()
	comp.Handle("/get/", a.Then(http.HandlerFunc(company.GetAPI))).Methods("GET")
	comp.Handle("/set/", admin.Then(http.HandlerFunc(company.SaveAPI))).Methods("POST")
	comp.Handle("/preview-receipt/", admin.Then(http.HandlerFunc(receipt.Preview))).Methods("GET")

	//app settings
	as := r.PathPrefix("/app-settings").Subrouter()
	as.Handle("/get/", a.Then(http.HandlerFunc(appsettings.GetAPI))).Methods("GET")
	as.Handle("/set/", admin.Then(http.HandlerFunc(appsettings.SaveAPI))).Methods("POST")
	as.Handle("/generate-api-key", admin.Then(http.HandlerFunc(appsettings.GenerateAPIKey))).Methods("GET")

	//serve static assets
	r.PathPrefix(staticWebDir).Handler(setStaticFileHeaders(http.StripPrefix(staticWebDir, http.FileServer(http.Dir(staticLocalDir)))))

	//serve anything off of the root directory
	//manifest.json, robots.txt, etc.
	r.PathPrefix("/").Handler(http.FileServer(http.Dir(staticLocalDir + "root_files/")))

	//Have the server listen
	log.Println("Starting stripe-appengine-frontend...")

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), r))
}

//parseAppYaml handles reading the app.yaml file
func parseAppYaml(path string) (yamlData appYaml, err error) {
	if len(path) < 1 {
		return appYaml{}, errors.New("no path was given for the app.yaml file")
	}

	fileData, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(fileData, &yamlData)
	if err != nil {
		return
	}

	//done
	return
}

//setStaticFileHeaders is used to set cache headers for static files
//this determines how long files will be cached on the client
func setStaticFileHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//max-age is seconds
		maxAge := cacheDays * 24 * 60 * 60
		w.Header().Set("Cache-Control", "no-transform,public,max-age="+strconv.Itoa(maxAge))

		//SERVE CONTENT
		h.ServeHTTP(w, r)
	})
}
