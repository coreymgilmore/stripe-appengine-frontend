/*
This file deals with receipts for charges.  Receipts are built using data from Stripe
on the charge and data for the company.  The company data just makes the receipt look legit.

This file specifically deals with showing the receipt for a charge.
*/

package receipt

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"google.golang.org/appengine"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/urlfetch"

	"chargeutils"
	"memcacheutils"
	"templates"
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
	info, err := getCompanyInfo(r)
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
