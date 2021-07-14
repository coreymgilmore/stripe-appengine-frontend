/*
Package receipt is used to generate and show a receipt for a specific credit card charge.
The data for a receipt is taken from Stripe (the charge data) and from the app engine datastore
(information on the company who runs this app). The company data is used to make the receipt look
legit.
*/
package receipt

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/appsettings"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/card"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/company"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/templates"
)

//receiptData is used for showing the receipt in html
type receiptData struct {
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
	Email,
	StatementDescriptor,

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

	//app settings
	Timezone string
}

//Show builds an html page that display a receipt
//this is a very boring, plain text, monospaced font page designed for easy printing and reading
//the receipt is generated from the charge id
//the data for the charge will have to be retrieved from stripe
func Show(w http.ResponseWriter, r *http.Request) {
	//get charge id from form value
	chargeID := r.FormValue("chg_id")

	//init stripe
	c := r.Context()
	sc := card.CreateStripeClient(c)

	//get charge data
	chg, err := sc.Charges.Get(chargeID, nil)
	if err != nil {
		fmt.Fprint(w, "An error occured and the receipt cannot be displayed.\n")
		fmt.Fprint(w, err)
		return
	}

	//extract charge data
	d := card.ExtractDataFromCharge(chg)

	//get company info
	companyInfo, _ := company.Get(r)
	if len(companyInfo.CompanyName) == 0 {
		companyInfo.CompanyName = "**Company info has not been set yet.**"
		companyInfo.Street = "**Please contact an administrator to fix this.**"
		log.Println("receipt.Show", "Cannot view receipt because company info hasn't been set yet.")
	}

	//reformat datetime
	utcLoc, err := time.LoadLocation("UTC")
	if err != nil {
		log.Println("receipt.Show: could not get UTC timezone location", err)
	}

	appData, err := appsettings.Get(r)
	timezone := "UTC" //default value
	if err != nil {
		log.Println("receipt.Show: could not get appsettings timezone", err)
	} else {
		timezone = appData.ReportTimezone
	}

	guiLoc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Println("receipt.Show: could not get gui timezone location", err)
	}

	originalTimeTime, err := time.ParseInLocation("2006-01-02T15:04:05.000Z", d.Timestamp, utcLoc)
	if d.Timestamp == "" {
		//when charge is authorized only, not capture
		d.Timestamp = "*not captured yet*"
	} else if err != nil {
		log.Println("receipt.Show: time reformat error", err)
	} else {
		d.Timestamp = originalTimeTime.In(guiLoc).Format("2006-01-02 @ 3:04:05PM")
	}

	//display receipt
	output := receiptData{
		CompanyName:         companyInfo.CompanyName,
		Street:              companyInfo.Street,
		Suite:               companyInfo.Suite,
		City:                companyInfo.City,
		State:               companyInfo.State,
		Postal:              companyInfo.PostalCode,
		Country:             companyInfo.Country,
		PhoneNum:            companyInfo.PhoneNum,
		Email:               companyInfo.Email,
		StatementDescriptor: companyInfo.StatementDescriptor,
		Customer:            d.Customer,
		Cardholder:          d.Cardholder,
		CardBrand:           d.CardBrand,
		LastFour:            d.LastFour,
		Expiration:          d.Expiration,
		Captured:            d.CapturedStr,
		Timestamp:           d.Timestamp,
		Amount:              d.AmountDollars,
		Invoice:             d.Invoice,
		Po:                  d.Po,
		Timezone:            appData.ReportTimezone,
	}
	templates.Load(w, "receipt", output)
}

//Preview shows a demo receipt with the company info and fake transaction data
//this is used to show the receipt when saving the company info.
func Preview(w http.ResponseWriter, r *http.Request) {
	//get company info
	companyInfo, err := company.Get(r)
	if err != nil {
		log.Println("receipt.Preview - Could not get company info", err)
	}
	if len(companyInfo.CompanyName) == 0 {
		companyInfo.CompanyName = "**Company info has not been set yet.**"
		companyInfo.Street = "**Please contact an administrator to fix this.**"
		log.Println("receipt.Preview", "Cannot preview receipt because company info hasn't been set yet.")
	}

	//get app settings (timezone)
	appInfo, err := appsettings.Get(r)
	if err != nil {
		log.Println("receipt.Preview - Could not get app settings", err)
		appInfo.ReportTimezone = "EST (just for preview)"
	}

	//display receipt
	output := receiptData{
		CompanyName:         companyInfo.CompanyName,
		Street:              companyInfo.Street,
		Suite:               companyInfo.Suite,
		City:                companyInfo.City,
		State:               companyInfo.State,
		Postal:              companyInfo.PostalCode,
		Country:             companyInfo.Country,
		PhoneNum:            companyInfo.PhoneNum,
		Email:               companyInfo.Email,
		StatementDescriptor: companyInfo.StatementDescriptor,
		Customer:            "ACME Dynamite Corp.",
		Cardholder:          "Wile E. Coyote",
		CardBrand:           "VISA",
		LastFour:            "4242",
		Expiration:          "01/2025",
		Captured:            "true",
		Timestamp:           "2025-01-02T08:16:32.000Z",
		Amount:              "256.04",
		Invoice:             "344402",
		Po:                  "3345",
		Timezone:            appInfo.ReportTimezone,
	}
	templates.Load(w, "receipt", output)
}
