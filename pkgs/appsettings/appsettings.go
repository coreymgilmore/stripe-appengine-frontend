/*
Package appsettings implements functions for changing settings of the
app.

App settings are anything that changes functionality of the app.
*/
package appsettings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/memcacheutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
)

//for referencing when looking up or setting data in datastore or memcache
//so we don't need to type in key names anywhere
const (
	memcacheKeyName = "app-settings-memcache-key"
	datastoreKind   = "appSettings"
	datastoreKey    = "appSettingsKey"
)

//ErrAppSettingsDoNotExist is thrown when no app settings exist yet
var ErrAppSettingsDoNotExist = errors.New("appsettings: info does not exist")

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

//Get actually retrienves the information from memcache or the datastore
//putting this into a separate func cleans up code elsewhere
func Get(r *http.Request) (result Settings, err error) {
	//check memcache
	c := appengine.NewContext(r)
	_, err = memcache.Gob.Get(c, memcacheKeyName, &result)
	if err == nil {
		return
	}

	//data not found in memcache
	//get from datastore
	if err == memcache.ErrCacheMiss {
		key := datastore.NewKey(c, datastoreKind, datastoreKey, 0, nil)

		//get data
		er := datastore.Get(c, key, &result)
		if er == datastore.ErrNoSuchEntity {
			//no app settings exist yet
			//return default values
			log.Infof(c, "%v", "App settings don't exist yet.  Returning default values.")
			result = defaultAppSettings
		}

		//save to memcache if results were found
		if er == nil {
			memcacheutils.Save(c, memcacheKeyName, result)
		}

		//make sure we don't return an error when data was found
		//or when data wasn't found and we just set the default values
		err = nil
	}

	return
}

//SaveAPI saves new or updates existing company info in the datastore
func SaveAPI(w http.ResponseWriter, r *http.Request) {
	//get form values
	reqCustID, _ := strconv.ParseBool(r.FormValue("requireCustID"))
	custIDFormat := strings.TrimSpace(r.FormValue("custIDFormat"))

	//context
	c := appengine.NewContext(r)
	log.Infof(c, "%+v", "getting app setting")

	//generate entity key
	//keyname is hard coded so only one entity exists
	key := datastore.NewKey(c, datastoreKind, datastoreKey, 0, nil)

	//build entity to save
	//or update existing entity
	data := Settings{}
	data.RequireCustomerID = reqCustID
	data.CustomerIDFormat = custIDFormat

	//save company info
	err := save(c, key, memcacheKeyName, data)
	if err != nil {
		output.Error(err, "", w, r)
		return
	}

	//done
	output.Success("dataSaved", data, w)
	return
}

//save does the actual saving to the datastore
func save(c context.Context, key *datastore.Key, memcacheKeyName string, d Settings) error {
	//save company info
	_, err := datastore.Put(c, key, &d)
	if err != nil {
		return err
	}

	//save company into to memcache
	//ignoring errors since we can always get data from the datastore
	memcacheutils.Save(c, memcacheKeyName, d)

	return nil
}

//SaveDefaultInfo sets some default data when a company first starts using this app
//This func is called when the initial super admin is created.
func SaveDefaultInfo(c context.Context) error {
	//generate entity key
	//keyname is hard coded so only one entity exists
	key := datastore.NewKey(c, datastoreKind, datastoreKey, 0, nil)

	//save
	err := save(c, key, memcacheKeyName, defaultAppSettings)
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
	var settings Settings
	c := appengine.NewContext(r)
	key := datastore.NewKey(c, datastoreKind, datastoreKey, 0, nil)
	err := datastore.Get(c, key, &settings)
	if err != nil {
		log.Infof(c, "Error occured looking up old api key", err)
		return
	}

	//set the new api key
	settings.APIKey = apiKey
	err = save(c, key, memcacheKeyName, settings)
	if err != nil {
		log.Infof(c, "Could not save new api key", err)
		return
	}

	output.Success("generateAPIKey", apiKey, w)
	return

}
