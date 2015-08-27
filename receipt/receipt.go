package receipt 

import (
	"net/http"
	"io/ioutil"
	"fmt"

	"appengine"
	"appengine/urlfetch"
	"appengine/memcache"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"

	"chargeutils"
	"memcacheutils"
	"templates"
)

const (
	PATH_COMPANY_NAME = "config/receipt/company-name.txt"
	PATH_STREET = 		"config/receipt/street.txt"
	PATH_CITY = 		"config/receipt/city.txt"
	PATH_STATE = 		"config/receipt/state.txt"
	PATH_POSTAL = 		"config/receipt/postal-code.txt"
	PATH_COUNTRY = 		"config/receipt/country.txt"
	PATH_PHONE_NUM = 	"config/receipt/phone-num.txt"
)

var (
	companyName = 	""
	street = 		""
	city = 			""
	state = 		""
	postal = 		""
	country = 		""
	phoneNum = 		""

	initError 		error
)

type templateData struct{
	CompanyName,
	Street,
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

//**********************************************************************
//INIT
//read company and address data from files to display in receipts
//save to variables
func Init() error {
	r, err := ioutil.ReadFile(PATH_COMPANY_NAME)
	if err != nil {
		initError = err
		return err
	}
	companyName = string(r)

	r, err = ioutil.ReadFile(PATH_STREET)
	if err != nil {
		initError = err
		return err
	}
	street = string(r)

	r, err = ioutil.ReadFile(PATH_CITY)
	if err != nil {
		initError = err
		return err
	}
	city = string(r)

	r, err = ioutil.ReadFile(PATH_STATE)
	if err != nil {
		initError = err
		return err
	}
	state = string(r)

	r, err = ioutil.ReadFile(PATH_POSTAL)
	if err != nil {
		initError = err
		return err
	}
	postal = string(r)

	r, err = ioutil.ReadFile(PATH_COUNTRY)
	if err != nil {
		initError = err
		return err
	}
	country = string(r)

	r, err = ioutil.ReadFile(PATH_PHONE_NUM)
	if err != nil {
		initError = err
		return err
	}
	phoneNum = string(r)

	return nil
}

//**********************************************************************
//HANDLE HTTP REQUESTS

//SHOW THE RECEIPT
//just a plain text page for easy printing and reading
func Show(w http.ResponseWriter, r *http.Request) {
	//get charge id from form value
	chargeId := r.FormValue("chg_id")

	//try looking up charge data in memcache
	var chg *stripe.Charge
	c := 		appengine.NewContext(r)
	_, err := 	memcache.Gob.Get(c, chargeId, &chg)
	
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

	//display receipt
	output := templateData{
		CompanyName: 	companyName,
		Street: 		street,
		City: 			city,
		State: 			state,
		Postal: 		postal,
		Country: 		country,
		PhoneNum: 		phoneNum,
		Customer: 		d.Customer,
		Cardholder: 	d.Cardholder,
		CardBrand: 		d.CardBrand,
		LastFour: 		d.LastFour,
		Expiration: 	d.Expiration,
		Captured: 		d.CapturedStr,
		Timestamp: 		d.Timestamp,
		Amount: 		d.AmountDollars,
		Invoice: 		d.Invoice,
		Po: 			d.Po,
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