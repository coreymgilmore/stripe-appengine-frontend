package receipt 

import (
	"net/http"
	"io/ioutil"
	"fmt"
	"strconv"
	"encoding/json"
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
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

	//look up charge data
	stripe.SetHTTPClient(urlfetch.Client(appengine.NewContext(r)))
	chg, err := charge.Get(chargeId, nil)
	if err != nil {
		fmt.Fprint(w, "An error occured and the receipt cannot be displayed.\n")
		fmt.Fprint(w, err)
		return
	}

	//get card information
	j, _ := 		json.Marshal(chg.Source)
	s := 			chg.Source
	s.UnmarshalJSON(j)
	card := 		s.Card
	cardholder := 	card.Name
	expMonth := 	strconv.FormatInt(int64(card.Month), 10)
	expYear := 		strconv.FormatInt(int64(card.Year), 10)
	exp := 			expMonth + "/" + expYear
	last4 := 		card.LastFour
	cardBrand := 	card.Brand

	//charge information
	amount := 		strconv.FormatFloat((float64(chg.Amount) / 100), 'f', 2, 64)
	timestamp := 	chg.Created

	//get metadata
	customerName := chg.Meta["customer_name"]
	invoice := 		chg.Meta["invoice_num"]
	po := 			chg.Meta["po_num"]

	//convert timestamp to dateteim
	dt := 			time.Unix(timestamp, 0).Format("2006-01-02T15:04:05.000Z")

	//display receipt
	fmt.Fprint(w, companyName + "\n")
	fmt.Fprint(w, street + "\n")
	fmt.Fprint(w, city + ", " + state + " " + postal + "\n")
	fmt.Fprint(w, country + "\n")
	fmt.Fprint(w, phoneNum + "\n")
	fmt.Fprint(w, "**************************************************\n")
	fmt.Fprint(w, "\n")
	
	fmt.Fprint(w, "Customer Name:        " + customerName + "\n")
	fmt.Fprint(w, "Cardholder:           " + cardholder + "\n")
	fmt.Fprint(w, "Card Type:            " + cardBrand + "\n")
	fmt.Fprint(w, "Card Ending:          " + last4 + "\n")
	fmt.Fprint(w, "Expiration:           " + exp + "\n")
	fmt.Fprint(w, "**************************************************\n")
	fmt.Fprint(w, "\n")

	fmt.Fprint(w, "Transaction Type:     Sale\n")
	fmt.Fprint(w, "Timestamp:            " + dt + "\n")
	fmt.Fprint(w, "**************************************************\n")
	fmt.Fprint(w, "\n")

	fmt.Fprint(w, "Amount Charged:       $" + amount + "\n")
	fmt.Fprint(w, "Invoice:              " + invoice + "\n")
	fmt.Fprint(w, "Purchase Order:       " + po + "\n")
	fmt.Fprint(w, "**************************************************\n")
	fmt.Fprint(w, "\n")

	return
}

//CHECK IF FILES WERE READ CORRECTLY
func Check() error {
	if initError != nil {
		return initError
	}

	return nil
}