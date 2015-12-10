package receipt

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"

	"chargeutils"
	"memcacheutils"
	"output"
	"templates"
)

const (
	MEMCACHE_KEY_COMP_INFO = "company-info-memcache-key"
	DATASTORE_KIND         = "companyInfo"
)

var (
	initError                  error
	ErrCompanyDataDoesNotExist = errors.New("companyInfoDoesNotExist")
)

//FOR SHOWING THE RECEIPT IN HTML
//used for building a template
type templateData struct {
	CompanyName,
	Street,
	Suite,
	City,
	State,
	Postal,
	Country,
	PhoneNum,
	Customer,
	Cardholder,
	CardBrand,
	LastFour,
	Expiration,
	Captured,
	Timestamp,
	Amount,
	Invoice,
	Po string
}

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
//HANDLE HTTP REQUESTS

//SHOW THE RECEIPT
//just a plain text page for easy printing and reading
//need to get data on the charge from stripe
//if this charge was just processed, it should be saved in memcache
//otherwise, get the charge data from stripe
func Show(w http.ResponseWriter, r *http.Request) {
	//get charge id from form value
	chargeId := r.FormValue("chg_id")

	//try looking up charge data in memcache
	var chg *stripe.Charge
	c := appengine.NewContext(r)
	_, err := memcache.Gob.Get(c, chargeId, &chg)

	//charge not found in memcache
	//look up charge data from stripe
	if err == memcache.ErrCacheMiss {
		//init stripe
		c := appengine.NewContext(r)
		stripe.SetBackend(stripe.APIBackend, nil)
		stripe.SetHTTPClient(urlfetch.Client(c))

		chg, err = charge.Get(chargeId, nil)
		if err != nil {
			fmt.Fprint(w, "An error occured and the receipt cannot be displayed.\n")
			fmt.Fprint(w, err)
			return
		}

		//save to memcache
		memcacheutils.Save(c, chg.ID, chg)
	}

	//extract charge data
	d := chargeutils.ExtractData(chg)

	//get company info from datastore
	//might also be in memcache
	_, info, err := getCompanyInfo(r)
	name, street, suite, city, state, postal, country, phone := "", "", "", "", "", "", "", ""
	if err == ErrCompanyDataDoesNotExist {
		name = "**Company info has not been set yet.**"
		street = "**Please contact an administrator to fix this.**"
	} else {
		name = info.CompanyName
		street = info.Street
		suite = info.Suite
		city = info.City
		state = info.State
		postal = info.PostalCode
		country = info.Country
		phone = info.PhoneNum
	}

	//display receipt
	output := templateData{
		CompanyName: name,
		Street:      street,
		Suite:       suite,
		City:        city,
		State:       state,
		Postal:      postal,
		Country:     country,
		PhoneNum:    phone,
		Customer:    d.Customer,
		Cardholder:  d.Cardholder,
		CardBrand:   d.CardBrand,
		LastFour:    d.LastFour,
		Expiration:  d.Expiration,
		Captured:    d.CapturedStr,
		Timestamp:   d.Timestamp,
		Amount:      d.AmountDollars,
		Invoice:     d.Invoice,
		Po:          d.Po,
	}
	templates.Load(w, "receipt", output)
	return
}

//**********************************************************************
//CHECK IF FILES WERE READ CORRECTLY
func Check() error {
	if initError != nil {
		return initError
	}

	return nil
}

//**********************************************************************
//GET AND SET RECEIPT/COMPANY INFO

//GET COMPANY INFO
//done when loading the change company info modal
//or when building the receipt page
func GetCompanyInfo(w http.ResponseWriter, r *http.Request) {
	_, info, err := getCompanyInfo(r)

	if err != nil {
		output.Error(err, "", w)
		return
	}

	output.Success("dataFound", info, w)
	return

}

//SAVE COMPANY INFO
func SaveCompanyInfo(w http.ResponseWriter, r *http.Request) {
	//get form values
	name := r.FormValue("name")
	street := r.FormValue("street")
	suite := r.FormValue("suite")
	city := r.FormValue("city")
	state := r.FormValue("state")
	postal := r.FormValue("postal")
	country := r.FormValue("country")
	phone := r.FormValue("phone")

	//init context
	c := appengine.NewContext(r)

	//get key from entity if this info is already saved
	//aka user is updating the company data
	intId, _, err := getCompanyInfo(r)
	if err != nil && err != ErrCompanyDataDoesNotExist {
		output.Error(err, "", w)
		return
	}

	//generate entity key from id if this entity already exists
	//otherwise generate a new key
	var key *datastore.Key
	if intId != 0 {
		key = datastore.NewKey(c, DATASTORE_KIND, "", intId, nil)
	} else {
		key = datastore.NewIncompleteKey(c, DATASTORE_KIND, nil)
	}

	//build entity to save
	//no real validation is needed since this info isnt used for much
	insert := companyInfo{
		CompanyName: name,
		Street:      street,
		Suite:       suite,
		City:        city,
		State:       strings.ToUpper(state),
		PostalCode:  postal,
		Country:     strings.ToUpper(country),
		PhoneNum:    phone,
	}

	//save company info
	_, err = datastore.Put(c, key, &insert)
	if err != nil {
		output.Error(err, "", w)
		return
	}

	//save company into to memcache
	memcacheutils.Save(c, MEMCACHE_KEY_COMP_INFO, insert)

	//done
	output.Success("dataSaved", insert, w)
	return
}

//**********************************************************************
//FUNCS

//GET COMPNAY INFO
//internal use
func getCompanyInfo(r *http.Request) (int64, companyInfo, error) {
	c := appengine.NewContext(r)

	//check memcached
	var result companyInfo
	_, err := memcache.Gob.Get(c, MEMCACHE_KEY_COMP_INFO, &result)
	if err == nil {
		return 0, result, nil

	} else if err == memcache.ErrCacheMiss {
		//look up data in datastore
		q := datastore.NewQuery(DATASTORE_KIND).Limit(1)
		r := make([]companyInfo, 0, 1)

		keys, err := q.GetAll(c, &r)
		if err != nil {
			return 0, companyInfo{}, err
		}

		//check if one result exists
		if len(r) == 0 {
			return 0, companyInfo{}, ErrCompanyDataDoesNotExist
		}

		//result does exist and was found
		info := r[0]

		//save to memcache
		memcacheutils.Save(c, MEMCACHE_KEY_COMP_INFO, info)

		//done
		return keys[0].IntID(), info, nil
	} else {
		return 0, companyInfo{}, err
	}
}
