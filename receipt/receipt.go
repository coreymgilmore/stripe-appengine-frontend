package receipt 

import (
	"net/http"
	"io/ioutil"
)

const (
	PATH_COMPANY_NAME = "config/receipt/company-name.txt"
	PATH_STREET = 		"config/receipt/street.txt"
	PATH_CITY = 		"config/receipt/city.txt"
	PATH_STATE = 		"config/receipt/state.txt"
	PATH_POSTAL = 		"config/receipt/postal-code.txt"
	PATH_PHONE_NUM = 	"config/receipt/phone-num.txt"
)

var (
	companyName = 	""
	street = 		""
	city = 			""
	state = 		""
	postal = 		""
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
	//get form values
	customerName := r.FormValue("cus")
	cardholder := 	r.FormValue("hol")
	last4 := 		r.FormValue("las")
	exp := 			r.FormValue("exp")
	amount := 		r.FormValue("amt")
	invoice := 		r.FormValue("inv")
	po := 			r.FormValue("pon")
	datetime := 	r.FormValue("dat")

	//display receipt
	w.Write([]byte(companyName + "\n"))
	w.Write([]byte(street + "\n"))
	w.Write([]byte(city + ", " + state + " " + postal + "\n"))
	w.Write([]byte(phoneNum + "\n"))
	w.Write([]byte("**************************************************\n"))
	w.Write([]byte("\n"))
	
	w.Write([]byte("Customer Name:        " + customerName + "\n"))
	w.Write([]byte("Cardholder:           " + cardholder + "\n"))
	w.Write([]byte("Card Ending:          " + last4 + "\n"))
	w.Write([]byte("Expiration:           " + exp + "\n"))
	w.Write([]byte("**************************************************\n"))
	w.Write([]byte("\n"))

	w.Write([]byte("Transaction Type:     Sale\n"))
	w.Write([]byte("Timestamp:            " + datetime + "\n"))
	w.Write([]byte("**************************************************\n"))
	w.Write([]byte("\n"))

	w.Write([]byte("Amount Charged:       $" + amount + "\n"))
	w.Write([]byte("Invoice:              " + invoice + "\n"))
	w.Write([]byte("Purchase Order:       " + po + "\n"))
	w.Write([]byte("**************************************************\n"))
	w.Write([]byte("\n"))

	return
}