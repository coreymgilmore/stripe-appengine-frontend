/*
Package appsettings implements functions for changing settings of the app.

App settings are anything that changes functionality of the app.
*/
package appsettings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
)

//for referencing when looking up or setting data in datastore
//so we don't need to type in key names anywhere
const (
	datastoreKind = "appSettings"
	datastoreKey  = "appSettingsKey"
)

//Settings is used for setting or getting the app settings from the datastore
type Settings struct {
	RequireCustomerID bool   `json:"require_cust_id"` //is the customer id field required when adding a new card
	CustomerIDFormat  string `json:"cust_id_format"`  //the format of the customer id from a CRM system.  maybe it  start swith CUST, or ACCT, etc.
	APIKey            string `json:"api_key"`         //the api key to access this app to automatically charge cards
}

//defaultAppSettings is the base configuration for the app
//default info
var defaultAppSettings = Settings{
	RequireCustomerID: false,
	CustomerIDFormat:  "",
	APIKey:            "",
}

//ErrAppSettingsDoNotExist is thrown when no app settings exist yet
var ErrAppSettingsDoNotExist = errors.New("appsettings: info does not exist")

//GetAPI is used when viewing the data in the gui or on a receipt
func GetAPI(w http.ResponseWriter, r *http.Request) {
	//get info
	info, err := Get(r)
	if err != nil {
		output.Error(err, "", w, r)
		return
	}

	output.Success("dataFound", info, w)
	return
}

//Get actually retrienves the information from the datastore
//putting this into a separate func cleans up code elsewhere
func Get(r *http.Request) (result Settings, err error) {
	//connect to datastore
	c := r.Context()
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return
	}

	//get the key we are looking up
	key := datastore.NameKey(datastoreKind, datastoreKey, nil)

	//get data
	err = client.Get(c, key, &result)
	if err == datastore.ErrNoSuchEntity {
		//no app settings exist yet
		//return default values
		log.Println("appsettings.Get", "App settings don't exist yet.  Returning default values.")
		result = defaultAppSettings
	}

	return
}

//SaveAPI saves new or updates existing company info in the datastore
func SaveAPI(w http.ResponseWriter, r *http.Request) {
	//get form values
	reqCustID, _ := strconv.ParseBool(r.FormValue("requireCustID"))
	custIDFormat := strings.TrimSpace(r.FormValue("custIDFormat"))

	//get the key we are saving to
	key := datastore.NameKey(datastoreKind, datastoreKey, nil)

	//build entity to save
	//or update existing entity
	data := Settings{}
	data.RequireCustomerID = reqCustID
	data.CustomerIDFormat = custIDFormat

	//save company info
	c := r.Context()
	err := save(c, key, data)
	if err != nil {
		output.Error(err, "", w, r)
		return
	}

	//done
	output.Success("dataSaved", data, w)
	return
}

//save does the actual saving to the datastore
func save(c context.Context, key *datastore.Key, d Settings) error {
	//connect to datastore
	client, err := datastoreutils.Connect(c)
	if err != nil {
		return err
	}

	//save company info
	_, err = client.Put(c, key, &d)
	if err != nil {
		return err
	}

	return nil
}

//SaveDefaultInfo sets some default data when a company first starts using this app
//This func is called when the initial super admin is created.
func SaveDefaultInfo(c context.Context) error {
	//generate entity key
	//keyname is hard coded so only one entity exists
	key := datastore.NameKey(datastoreKind, datastoreKey, nil)

	//save
	err := save(c, key, defaultAppSettings)
	return err
}

//GenerateAPIKey creates a new api key and saves it to the datastore
//the key is also returned to update the gui
//limit api key length so it is easier to use
//multiple calls to this func will "rotate" the api key
func GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	//generate a new api key
	//just a simple sha256 string off the current time
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	h := sha256.New()
	h.Write([]byte(ts))
	apiKey := strings.ToUpper(hex.EncodeToString(h.Sum(nil))[:20])

	//get the existing api key to update
	settings, err := Get(r)
	if err != nil {
		output.Error(err, "", w, r)
		return
	}

	//get the key we are saving to
	key := datastore.NameKey(datastoreKind, datastoreKey, nil)

	//set the new api key
	settings.APIKey = apiKey
	c := r.Context()
	err = save(c, key, settings)
	if err != nil {
		log.Println("Could not save new api key", err)
		return
	}

	output.Success("generateAPIKey", apiKey, w)
	return
}
