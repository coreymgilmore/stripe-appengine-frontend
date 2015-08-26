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
		stripe.SetHTTPClient(urlfetch.Client(appengine.NewContext(r)))
		chg, err = charge.Get(chargeId, nil)
		if err != nil {
			fmt.Fprint(w, "An error occured and the receipt cannot be displayed.\n")
			fmt.Fprint(w, err)
			return
		}
	}

	//extract charge data
	d := chargeutils.ExtractData(chg)

	//display receipt
	fmt.Fprint(w, companyName + "\n")
	fmt.Fprint(w, street + "\n")
	fmt.Fprint(w, city + ", " + state + " " + postal + "\n")
	fmt.Fprint(w, country + "\n")
	fmt.Fprint(w, phoneNum + "\n")
	fmt.Fprint(w, "**************************************************\n")
	fmt.Fprint(w, "\n")
	
	fmt.Fprint(w, "Customer Name:        " + d.Customer + "\n")
	fmt.Fprint(w, "Cardholder:           " + d.Cardholder + "\n")
	fmt.Fprint(w, "Card Type:            " + d.CardBrand + "\n")
	fmt.Fprint(w, "Card Ending:          " + d.LastFour + "\n")
	fmt.Fprint(w, "Expiration:           " + d.Expiration + "\n")
	fmt.Fprint(w, "**************************************************\n")
	fmt.Fprint(w, "\n")

	fmt.Fprint(w, "Transaction Type:     Sale\n")
	fmt.Fprint(w, "Captured:             " + d.CapturedStr + "\n")
	fmt.Fprint(w, "Timestamp (UTC):      " + d.Timestamp + "\n")
	fmt.Fprint(w, "**************************************************\n")
	fmt.Fprint(w, "\n")

	fmt.Fprint(w, "Amount Charged:       $" + d.AmountDollars + "\n")
	fmt.Fprint(w, "Invoice:              " + d.Invoice + "\n")
	fmt.Fprint(w, "Purchase Order:       " + d.Po + "\n")
	fmt.Fprint(w, "**************************************************\n")
	fmt.Fprint(w, "\n")

	//error checking
	if d.Captured == false {
		fmt.Fprint(w, "\n\n\n")
		fmt.Fprint(w, "**************************************************\n")
		fmt.Fprint(w, "**************************************************\n")
		fmt.Fprint(w, "ERROR!   ERROR!   ERROR!\n")
		fmt.Fprint(w, "Charge was not captured!")
	}

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