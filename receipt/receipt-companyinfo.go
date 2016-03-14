/*
This file deals with setting, updating, ad getting the company info that is displayed on a receipt.
The company info is just used to make the receipt look legit and have some helpful info.
*/

package receipt

import (
	"net/http"
	"strings"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"

	"memcacheutils"
	"output"
)

const (
	//for referencing when looking up or setting data in datastore or memcache
	//so we don't need to type in key names anywhere
	memcacheKeyName = "company-info-memcache-key"
	datastoreKind   = "companyInfo"
	datastoreKey    = "companyInfoKey"
)

//FOR GETTING DATA FROM DATASTORE
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

//**********************************************************************
//GET AND SET RECEIPT/COMPANY INFO

//GET COMPANY INFO
//done when loading the change company info modal
//or when building the receipt page
func GetCompanyInfo(w http.ResponseWriter, r *http.Request) {
	info, err := getCompanyInfo(r)

	if err != nil {
		output.Error(err, "", w)
		return
	}

	output.Success("dataFound", info, w)
	return

}

//SAVE COMPANY INFO
//this updates existing info if it exists
func SaveCompanyInfo(w http.ResponseWriter, r *http.Request) {
	//GET FORM VALUES
	name := strings.TrimSpace(r.FormValue("name"))
	street := strings.TrimSpace(r.FormValue("street"))
	suite := strings.TrimSpace(r.FormValue("suite"))
	city := strings.TrimSpace(r.FormValue("city"))
	state := strings.TrimSpace(r.FormValue("state"))
	postal := strings.TrimSpace(r.FormValue("postal"))
	country := strings.TrimSpace(r.FormValue("country"))
	phone := strings.TrimSpace(r.FormValue("phone"))

	//LOOK UP DATA FOR THIS COMPANY
	//may return a blank struct if this data does not exist yet
	data, err := getCompanyInfo(r)
	if err != nil && err != ErrCompanyDataDoesNotExist {
		output.Error(err, "", w)
		return
	}

	//CONTEXT
	c := appengine.NewContext(r)

	//GENERATE ENTITY KEY
	//keyname is hard coded so only one entity exists
	key := datastore.NewKey(c, datastoreKind, datastoreKey, 0, nil)

	//BUILD ENTITY TO SAVE
	//update existing entity
	data.CompanyName = name
	data.Street = street
	data.Suite = suite
	data.City = city
	data.State = strings.ToUpper(state)
	data.PostalCode = postal
	data.Country = strings.ToUpper(country)
	data.PhoneNum = phone

	//SAVE COMPANY INFO
	_, err = datastore.Put(c, key, &data)
	if err != nil {
		output.Error(err, "", w)
		return
	}

	//SAVE COMPANY INTO TO MEMCACHE
	memcacheutils.Save(c, memcacheKeyName, data)

	//done
	output.Success("dataSaved", data, w)
	return
}

//**********************************************************************
//FUNCS

//GET COMPANY INFO
func getCompanyInfo(r *http.Request) (companyInfo, error) {
	c := appengine.NewContext(r)

	//CHECK MEMCACHED
	var result companyInfo
	_, err := memcache.Gob.Get(c, memcacheKeyName, &result)
	if err == nil {
		//DATA FOUND IN MEMCACHE
		//return it
		return result, nil

		//LOOK FOR DATA IN DATASTORE
		//since data was not in memache
	} else if err == memcache.ErrCacheMiss {
		//GENERATE KEY TO LOOK UP DATA
		key := datastore.NewKey(c, datastoreKind, datastoreKey, 0, nil)

		//GET DATA
		err := datastore.Get(c, key, &result)
		if err == datastore.ErrNoSuchEntity {
			return result, ErrCompanyDataDoesNotExist
		} else if err != nil {
			return result, err
		}

		//DATA FOUND, SAVE TO MEMCACHE
		memcacheutils.Save(c, memcacheKeyName, result)

		//RETURN DATA
		return result, nil

		//UNKNOWN ERROR
	} else {
		return companyInfo{}, err
	}
}
