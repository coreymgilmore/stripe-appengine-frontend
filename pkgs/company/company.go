/*
Package company implements tools for handling your company's information.  This is
for the company name and address, the amount of fees you pay per transaction to
Stripe, and the text that is displayed on the credit card statement.

Company data is anything for the company: address, contact info, receipt,
statement description, and fees.
*/
package company

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	memcacheKeyName = "company-info-memcache-key"
	datastoreKind   = "companyInfo"
	datastoreKey    = "companyInfoKey"
)

//maxStatementDescriptorLength is the maximum length of the statement description
//this is dictated by Stripe
const maxStatementDescriptorLength = 22

//ErrCompanyDataDoesNotExist is thrown when no company data has been set yet
//this occurs when an admin did not go into the settings and provide the company info
var ErrCompanyDataDoesNotExist = errors.New("company: info does not exist")

//Info is used for setting or getting the company data from the datastore
type Info struct {
	CompanyName         string  `json:"company_name"`         //for receipts
	Street              string  `json:"street"`               // " "
	Suite               string  `json:"suite"`                // " "
	City                string  `json:"city"`                 // " "
	State               string  `json:"state"`                // " "
	PostalCode          string  `json:"postal_code"`          // " "
	Country             string  `json:"country"`              // " "
	PhoneNum            string  `json:"phone_num"`            // " "
	Email               string  `json:"email"`                // " "
	PercentFee          float64 `json:"percentage_fee"`       //default is 2.90% transaction per Stripe
	FixedFee            float64 `json:"fixed_fee"`            //default is $0.30 per transaction per Stripe
	StatementDescriptor string  `json:"statement_descriptor"` //what is displayed on the statement with our charge
}

//defaultCompanyInfo is the minimal amount of info required
var defaultCompanyInfo = Info{
	CompanyName:         "",
	Street:              "",
	Suite:               "",
	City:                "",
	State:               "",
	PostalCode:          "",
	Country:             "",
	PhoneNum:            "",
	Email:               "",
	PercentFee:          .0290,
	FixedFee:            0.30,
	StatementDescriptor: "",
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
func Get(r *http.Request) (result Info, err error) {
	//check memcache
	c := r.Context(r)
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
			//no company info exists yet
			//return default values
			log.Infof(c, "%v", "Company info doesn't exist yet.  Returning default values.")
			result = defaultCompanyInfo
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
	name := strings.TrimSpace(r.FormValue("name"))
	street := strings.TrimSpace(r.FormValue("street"))
	suite := strings.TrimSpace(r.FormValue("suite"))
	city := strings.TrimSpace(r.FormValue("city"))
	state := strings.TrimSpace(r.FormValue("state"))
	postal := strings.TrimSpace(r.FormValue("postal"))
	country := strings.TrimSpace(r.FormValue("country"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	email := strings.TrimSpace(r.FormValue("email"))
	percentFee, _ := strconv.ParseFloat(r.FormValue("percentFee"), 64)
	fixedFee, _ := strconv.ParseFloat(r.FormValue("fixedFee"), 64)
	statementDesc := strings.TrimSpace(r.FormValue("descriptor"))

	//context
	c := r.Context(r)

	//generate entity key
	//keyname is hard coded so only one entity exists
	key := datastore.NewKey(c, datastoreKind, datastoreKey, 0, nil)

	//shorten up statement descriptor if needed
	if len(statementDesc) > maxStatementDescriptorLength {
		statementDesc = statementDesc[:maxStatementDescriptorLength]
	}

	//save the percentage fee as a decimal number with up to 4 decimal places
	//2.85% = 0.0285
	log.Debugf(c, "%v", percentFee)

	percentFeeStr := "0.0" + strconv.FormatFloat(percentFee*100, 'f', 0, 64)
	percentFee, _ = strconv.ParseFloat(percentFeeStr, 64)

	log.Debugf(c, "%v", percentFeeStr)
	log.Debugf(c, "%v", percentFee)

	//build entity to save
	//or update existing entity
	data := Info{}
	data.CompanyName = name
	data.Street = street
	data.Suite = suite
	data.City = city
	data.State = strings.ToUpper(state)
	data.PostalCode = postal
	data.Country = strings.ToUpper(country)
	data.PhoneNum = phone
	data.Email = email
	data.PercentFee = percentFee
	data.FixedFee = fixedFee
	data.StatementDescriptor = statementDesc

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
func save(c context.Context, key *datastore.Key, memcacheKeyName string, d Info) error {
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
	err := save(c, key, memcacheKeyName, defaultCompanyInfo)
	return err
}
