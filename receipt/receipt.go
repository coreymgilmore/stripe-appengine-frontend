/*
Package receipt is used to generate and show a receipt for a specific credit card charge.
The data for a receipt is taken from Stripe (the charge data) and from the app engine datastore
(information on the company who runs this app). The company data is used to make the receipt look
legit.
*/

package receipt

import (
	"chargeutils"
	"errors"
	"fmt"
	"memcacheutils"
	"net/http"
	"templates"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"google.golang.org/appengine"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/urlfetch"
)

//ErrCompanyDataDoesNotExist is thrown when no company data has been set yet
//this occurs when an admin did not go into the settings and provide the company info
var ErrCompanyDataDoesNotExist = errors.New("companyInfoDoesNotExist")

//templateData is used for showing the receipt in html
type templateData struct {
	//information about the company that uses this app
	//"your" company, not the company for the card
	CompanyName,
	Street,
	Suite,
	City,
	State,
	Postal,
	Country,
	PhoneNum,

	//information about the card that was charged
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

//Show builds an html page that display a receipt
//this is a very boring, plain test, monospaced font page designed for easy printing and reading
//the receipt is generated from the charge id
//the data for the charge may be in memcache or will have to be retrieved from stripe
func Show(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	//get charge id from form value
	chargeID := r.FormValue("chg_id")

	//try looking up charge data in memcache
	var chg *stripe.Charge
	_, err := memcache.Gob.Get(c, chargeID, &chg)

	//charge not found in memcache
	//look up charge data from stripe
	if err == memcache.ErrCacheMiss {
		//init stripe
		c := appengine.NewContext(r)
		stripe.SetBackend(stripe.APIBackend, nil)
		stripe.SetHTTPClient(urlfetch.Client(c))

		//get charge data
		chg, err = charge.Get(chargeID, nil)
		if err != nil {
			fmt.Fprint(w, "An error occured and the receipt cannot be displayed.\n")
			fmt.Fprint(w, err)
			return
		}

		//save to memcache
		//just in case we want to view the receipt again
		memcacheutils.Save(c, chg.ID, chg)
	}

	//extract charge data
	d := chargeutils.ExtractData(chg)

	//get company info
	info, err := getCompanyInfo(r)
	var name, street, suite, city, state, postal, country, phone string
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
