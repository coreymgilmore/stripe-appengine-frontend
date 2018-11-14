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
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sqliteutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/templates"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	yaml "gopkg.in/yaml.v2"
)

//defaultPort is the port used when serving in the dev environment
const defaultPort = "8005"

//staticWebDir is the directory off of the root domain from which static files are served
//www.example.com/static/ -? /static
//leave off trailing slash
const staticWebDir = "/static"

//staticLocalDir is the location on the server's filesystem where we store static files
var staticLocalDir = ""

//cacheDays is the number of days to cache static files
//this is set in init() and used when serving static files
//0 (zero) means don't cache files at all
var cacheDays = 0

//appYaml is the format of the app.yaml file
type appYaml struct {
	Runtime string `yaml:"runtime"` //should be go111 for deployments to appengine
	EnvVars struct {
		ProjectID            string `yaml:"PROJECT_ID"`                 //the project id on google cloud
		SessionAuthKey       string `yaml:"SESSION_AUTH_KEY"`           //session cookie
		SessionEncryptKey    string `yaml:"SESSION_ENCRYPT_KEY"`        //session cookie
		SessionLifetime      int    `yaml:"SESSION_LIFETIME"`           //how many days a user will remain logged in for
		CookieDomain         string `yaml:"COOKIE_DOMAINCOOKIE_DOMAIN"` //the domain the session cookie is used for
		StripeSecretKey      string `yaml:"STRIPE_SECRET_KEY"`          //used for charging cards
		StripePublishableKey string `yaml:"STRIPE_PUBLISHABLE_KEY"`     //used for creating customers and saving cards
		CacheDays            int    `yaml:"CACHE_DAYS"`                 //number of days to cache static files
		StaticFilePath       string `yaml:"PATH_TO_STATIC_FILES"`       //the full path to the ./website/static/ directory
		TemplatesPath        string `yaml:"PATH_TO_TEMPLATES"`          //the full path to the templates directory
		UseLocalFiles        string `yaml:"USE_LOCAL_FILES"`            //true serves css/js/fonts from local domain versus cdn
		PathToSqliteFile     string `yaml:"PATH_TO_SQLITE_FILE"`        //the full path to the file used for the sqlite db.  If blank, the default path is used
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

//parsedYamlData is the data parsed from the app.yaml file
//we parse the data in app.yaml and store it here so we can use it in diag()
var parsedAppYaml appYaml

//flags
var (
	deploymentType             string
	pathToAppYaml              string
	pathToDatastoreCredentials string
	useDevDatastore            bool
)

//these are the type of deployments we support
const (
	deploymentTypeAppengine    = "appengine"
	deploymentTypeAppengineDev = "appengine-dev"
	deploymentTypeSqlite       = "sqlite"
)

func init() {
	//use flags to allow for different deployment types
	//deploymentType: used for changing how this app is deployed:
	//  - appengine,
	//  - appengine-dev (locally running but using Cloud datastore),
	//  - sqlite (future)
	//pathToAppYaml: the full path to the app.yaml file.  This file is
	//  used for deployment types other than appengine-dev to get
	//  environmental variables.  This eliminates the need to set
	//  these variables when deploying or testing locally.
	//pathToDatastoreCredentials: the full path to the credentials file used
	//  to connect to the Cloud Datastore when deployed locally or testing.
	//  This file is downloaded from the GCP Console when creating a service
	//  account.
	//  This is *not* set in an environmental variable since when deployed to
	//  appengine it isn't needed, when testing locally it is in the same directry
	//  as main.go which when run as "go run main.go" makes the credentials file easy
	//  to find, and when deployed via sqlite the datastore credentials aren't needed.
	//useDevDatastore: this overrides the default "true" value of using the dev datastore
	//  when in dev mode.  Sometimes you may want to use live/production data even
	//  though you are developing (for example, dev environment doesn't have any data).
	flag.StringVar(&deploymentType, "type", "appengine", "Set to appengine or appengine-dev.  In development mode the app.yaml file will be parsed to read the set environmental variables.")
	flag.StringVar(&pathToAppYaml, "path-to-app-yaml", "./app.yaml", "The path to the app.yaml file.")
	flag.StringVar(&pathToDatastoreCredentials, "path-to-datastore-credentials", "./datastore-service-account.json", "The path to your datastore service account file.  A JSON file.")
	flag.BoolVar(&useDevDatastore, "use-dev-datastore", true, "Not used for non -dev deployment types. Set to false to use live datastore data in development deployment types.")
	flag.Parse()

	//set configuration options based on deployment type
	switch deploymentType {
	case deploymentTypeAppengine:
		//the default deployment type
		//when this app is run on appengine, the environmental variables in app.yaml file will automatically be provided to the app
		c := sessionutils.Config
		c.SessionAuthKey = os.Getenv("SESSION_AUTH_KEY")
		c.SessionEncryptKey = os.Getenv("SESSION_ENCRYPT_KEY")
		c.SessionLifetime, _ = strconv.Atoi(os.Getenv("SESSION_LIFETIME"))
		c.CookieDomain = os.Getenv("COOKIE_DOMAIN")
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

		cccc := templates.Config
		cccc.PathToTemplates = "./website/templates/"
		cccc.Development = false
		cccc.UseLocalFiles, _ = strconv.ParseBool(os.Getenv("USE_LOCAL_FILES"))
		templates.SetConfig(cccc)

		//set path to static files
		//default for appengine deployments
		staticLocalDir = "./website/static/"

		//set cache max age
		cacheDays, _ = strconv.Atoi(os.Getenv("CACHE_DAYS"))

		//save data for diagnostics
		parsedAppYaml.EnvVars.ProjectID = os.Getenv("PROJECT_ID")
		parsedAppYaml.EnvVars.SessionLifetime, _ = strconv.Atoi(os.Getenv("SESSION_LIFETIME"))
		parsedAppYaml.EnvVars.CacheDays, _ = strconv.Atoi(os.Getenv("CACHE_DAYS"))
		parsedAppYaml.EnvVars.UseLocalFiles = os.Getenv("USE_LOCAL_FILES")
		parsedAppYaml.EnvVars.CookieDomain = os.Getenv("COOKIE_DOMAIN")
		useDevDatastore = false

	case deploymentTypeAppengineDev:
		//check for and parse the app.yaml file
		yamlData, err := parseAppYaml(pathToAppYaml)
		if err != nil {
			log.Fatalln("Error while parsing app.yaml.", err)
			return
		}

		//saved parsed data for use elsewhere
		parsedAppYaml = yamlData

		//set configuration options using app.yaml
		//this is how we set the configuration in development
		c := sessionutils.Config
		c.SessionAuthKey = yamlData.EnvVars.SessionAuthKey
		c.SessionEncryptKey = yamlData.EnvVars.SessionEncryptKey
		c.SessionLifetime = yamlData.EnvVars.SessionLifetime
		c.CookieDomain = yamlData.EnvVars.CookieDomain
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
		//need to set an environmental variable here since it provide credentials to the datastore
		//see https://cloud.google.com/datastore/docs/reference/libraries#client-libraries-install-go
		err = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", pathToDatastoreCredentials)
		if err != nil {
			log.Fatalln("Could not set Google Datastore credentials environmental variable.", err)
			return
		}

		//by default, this will connect to the Cloud datastore.
		//if you want to use a local development datastore, see https://cloud.google.com/datastore/docs/tools/datastore-emulator
		//during development, we use dev entities or kinds.  The entity types are prepended "dev-" to keep dev data separate.
		ccc := datastoreutils.Config
		ccc.ProjectID = yamlData.EnvVars.ProjectID
		ccc.Development = useDevDatastore
		err = datastoreutils.SetConfig(ccc)
		if err != nil {
			log.Fatalln("Could not set configuration for datastore.", err)
			return
		}

		cccc := templates.Config
		cccc.PathToTemplates = yamlData.EnvVars.TemplatesPath
		cccc.Development = true
		cccc.UseLocalFiles, _ = strconv.ParseBool(yamlData.EnvVars.UseLocalFiles)
		templates.SetConfig(cccc)

		//set path to static files
		//default for appengine deployments
		staticLocalDir = yamlData.EnvVars.StaticFilePath

		//set cache max age
		cacheDays = yamlData.EnvVars.CacheDays

	case deploymentTypeSqlite:
		//a version of this app that can run without appengine and is backed by sqlite
		//check for and parse the app.yaml file
		yamlData, err := parseAppYaml(pathToAppYaml)
		if err != nil {
			log.Fatalln("Error while parsing app.yaml.", err)
			return
		}

		//saved parsed data for use elsewhere
		parsedAppYaml = yamlData

		//set configuration options using app.yaml
		//this is how we set the configuration in development
		c := sessionutils.Config
		c.SessionAuthKey = yamlData.EnvVars.SessionAuthKey
		c.SessionEncryptKey = yamlData.EnvVars.SessionEncryptKey
		c.SessionLifetime = yamlData.EnvVars.SessionLifetime
		c.CookieDomain = yamlData.EnvVars.CookieDomain
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

		ccc := sqliteutils.Config
		ccc.PathToDatabaseFile = yamlData.EnvVars.PathToSqliteFile
		sqliteutils.SetConfig(ccc)
		sqliteutils.Connect()

		cccc := templates.Config
		cccc.PathToTemplates = yamlData.EnvVars.TemplatesPath
		cccc.Development = true
		cccc.UseLocalFiles, _ = strconv.ParseBool(yamlData.EnvVars.UseLocalFiles)
		templates.SetConfig(cccc)

		//set path to static files
		//default for appengine deployments
		staticLocalDir = yamlData.EnvVars.StaticFilePath

		//set cache max age
		cacheDays = yamlData.EnvVars.CacheDays

	default:
		//when an invalid deployment type is given
		log.Fatalln("An invalid deployment type was given as a flag.")
		return
	}

	//make sure cache days is a valid value
	if cacheDays < 0 {
		log.Fatalln("Cache days value should be a value greater than or equal to zero.")
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
	r.Handle("/main/", a.Then(http.HandlerFunc(pages.Main)))
	r.Handle("/diag/", http.HandlerFunc(diag))

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
	r.PathPrefix("/").Handler(http.FileServer(http.Dir(staticLocalDir + "root-files/")))

	//Have the server listen
	log.Println("Starting stripe-appengine-frontend...")

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	log.Println("Listening on port:", port)
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

//diag shows a diagnostic page with info on this app
func diag(w http.ResponseWriter, r *http.Request) {

	d := map[string]string{
		"Cookie Domain":                      parsedAppYaml.EnvVars.CookieDomain,
		"Deployment Type":                    deploymentType,
		"Session Lifetime (days)":            strconv.Itoa(parsedAppYaml.EnvVars.SessionLifetime),
		"Static File Cache Lifetime (days)":  strconv.Itoa(parsedAppYaml.EnvVars.CacheDays),
		"Use Development Database/Datastore": strconv.FormatBool(useDevDatastore),
		"Use Local Files":                    parsedAppYaml.EnvVars.UseLocalFiles,

		//appengine specific stuff
		//when deployement type = appengine, these fields will have values.  otherwise they are blank
		"Project ID":                 parsedAppYaml.EnvVars.ProjectID,
		"App Engine Service Name":    os.Getenv("GAE_SERVICE"),
		"App Engine Service Version": os.Getenv("GAE_VERSION"),
		"App Engine Instance ID":     os.Getenv("GAE_INSTANCE"),

		//appengine-dev or sqlite stuff
		//when app is installed in a non-appengine environment
		"Path to Datastore Credentials": pathToDatastoreCredentials,
		"Path to Static Files":          parsedAppYaml.EnvVars.StaticFilePath,
		"Path to Templates":             parsedAppYaml.EnvVars.TemplatesPath,
	}

	templates.Load(w, "diagnostics", d)
	return
}
