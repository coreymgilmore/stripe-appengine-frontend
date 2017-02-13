/*
Package receipt is used to generate and show a receipt for a specific credit card charge.
The data for a receipt is taken from Stripe (the charge data) and from the app engine datastore
(information on the company who runs this app). The company data is used to make the receipt look
legit.

This file deals with setting, updating, ad getting the company info that is displayed on a receipt.
The company info is just used to make the receipt look legit and have some helpful info.
*/

package receipt

import (
	"memcacheutils"
	"net/http"
	"output"
	"strings"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
)

//for referencing when looking up or setting data in datastore or memcache
//so we don't need to type in key names anywhere
const (
	memcacheKeyName = "company-info-memcache-key"
	datastoreKind   = "companyInfo"
	datastoreKey    = "companyInfoKey"
)

//companyInfo is used for setting or getting the company data from the datastore
type companyInfo struct {
	CompanyName string `json:"company_name"`
	Street      string `json:"street"`
	Suite       string `json:"suite"`
	City        string `json:"city"`
	State       string `json:"state"`
	PostalCode  string `json:"postal_code"`
	Country     string `json:"country"`
	PhoneNum    string `json:"phone_num"`
}

//GetCompanyInfo is used when viewing the data in the gui or on a receipt
func GetCompanyInfo(w http.ResponseWriter, r *http.Request) {
	//get info
	info, err := getCompanyInfo(r)
	if err != nil {
		output.Error(err, "", w, r)
		return
	}

	output.Success("dataFound", info, w)
	return

}

//SaveCompanyInfo saves new or updates existing company info in the datastore
func SaveCompanyInfo(w http.ResponseWriter, r *http.Request) {
	//get form values
	name := strings.TrimSpace(r.FormValue("name"))
	street := strings.TrimSpace(r.FormValue("street"))
	suite := strings.TrimSpace(r.FormValue("suite"))
	city := strings.TrimSpace(r.FormValue("city"))
	state := strings.TrimSpace(r.FormValue("state"))
	postal := strings.TrimSpace(r.FormValue("postal"))
	country := strings.TrimSpace(r.FormValue("country"))
	phone := strings.TrimSpace(r.FormValue("phone"))

	//look up data for this company
	//may return a blank struct if this data does not exist yet
	data, err := getCompanyInfo(r)
	if err != nil && err != ErrCompanyDataDoesNotExist {
		output.Error(err, "", w, r)
		return
	}

	//context
	c := appengine.NewContext(r)

	//generate entity key
	//keyname is hard coded so only one entity exists
	key := datastore.NewKey(c, datastoreKind, datastoreKey, 0, nil)

	//build entity to save
	//or update existing entity
	data.CompanyName = name
	data.Street = street
	data.Suite = suite
	data.City = city
	data.State = strings.ToUpper(state)
	data.PostalCode = postal
	data.Country = strings.ToUpper(country)
	data.PhoneNum = phone

	//save company info
	_, err = datastore.Put(c, key, &data)
	if err != nil {
		output.Error(err, "", w, r)
		return
	}

	//save company into to memcache
	//ignoring errors since we can always get data from the datastore
	memcacheutils.Save(c, memcacheKeyName, data)

	//done
	output.Success("dataSaved", data, w)
	return
}

//getCompanyInfo actually retrienves the information from memcache or the datastore
//putting this into a separate func cleans up code elsewhere
func getCompanyInfo(r *http.Request) (companyInfo, error) {
	//check memcache
	c := appengine.NewContext(r)
	var result companyInfo
	_, err := memcache.Gob.Get(c, memcacheKeyName, &result)
	if err == nil {
		return result, nil
	}

	//data not found in memcache
	//get from datastore
	if err == memcache.ErrCacheMiss {
		key := datastore.NewKey(c, datastoreKind, datastoreKey, 0, nil)

		//get data
		err := datastore.Get(c, key, &result)
		if err == datastore.ErrNoSuchEntity {
			return result, ErrCompanyDataDoesNotExist
		} else if err != nil {
			return result, err
		}

		//save to memcache
		//ignore errors since we already got the data
		memcacheutils.Save(c, memcacheKeyName, result)

		//return data
		return result, nil

	} else if err != nil {
		return companyInfo{}, err
	}

	return companyInfo{}, err
}
