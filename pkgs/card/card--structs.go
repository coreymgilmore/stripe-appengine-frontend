/*
This file is part of the card package.  This file just holds the structs needed for handling cards.
These structs are in a separate file to clean up the other files.

*/
package card

import (
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/chargeutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
)

//CustomerDatastore is the format for data being saved to the datastore when a new customer is added
//this data is also returned when looking up a customer
type CustomerDatastore struct {
	//the CRM id of the customer
	//used when performing semi-automated api style requests to this app
	//this should be unique for every customer but can be left blank
	CustomerID string `datastore:"CustomerId" json:"customer_id"`

	//the name of the customer
	//usually this is the company an individual card holder works for
	CustomerName string `json:"customer_name"`

	//the name on the card
	Cardholder string `json:"cardholder_name"`

	//MM/YYYY
	//used to just show in the gui which card is on file
	CardExpiration string `json:"card_expiration"`

	//used to just show in the gui which card is on file
	CardLast4 string `json:"card_last4"`

	//the id returned when a card is saved via Stripe
	//this id uniquely identifies this card for this customer
	StripeCustomerToken string `json:"-"`

	//when was this card added to the app
	DatetimeCreated string `json:"-"`

	//which user of the app saved the card
	//used for diagnostics in the cloud platform console
	AddedByUser string `json:"added_by"`
}

//chargeSuccessful is used to return data to the gui when a charge is processed
//this data shows which card was processed and some confirmation details.
type chargeSuccessful struct {
	CustomerName   string `json:"customer_name"`
	Cardholder     string `json:"cardholder_name"`
	CardExpiration string `json:"card_expiration"`
	CardLast4      string `json:"card_last4"`

	//the amount of the charge as a dollar amount string
	Amount string `json:"amount"`

	//the invoice number to reference for this charge
	Invoice string `json:"invoice"`

	//the po number to reference for this charge
	Po string `json:"po"`

	//when the charge was processed
	Datetime string `json:"datetime"`

	//the id returned by stripe for this charge
	//used to show a receipt if needed
	ChargeID string `json:"charge_id"`
}

//CardList is used to return the list of cards available to be charged to build the gui
//the list is filled into a datalist in html to form an autocomplete list
//the app user chooses a card from this list to remove or process a charge
type CardList struct {
	//company name for the card
	CustomerName string `json:"customer_name"`

	//the App Engine datastore id of the card
	//this is what uniquely identifies the card in the app engine datatore so we can look up the stripe customer token to process a charge
	ID int64 `json:"id"`
}

//reportData is used to build the report UI
type reportData struct {
	//The data for the logged in user
	//so we can show/hide certain UI elements based on the user's access rights
	UserData users.User `json:"user_data"`

	//The datetime we are filtering for getting report data
	//to limit the days a report gets data for
	StartDate time.Time `json:"start_datetime"`
	EndDate   time.Time `json:"end_datetime"`

	//Data for each charge for the report
	//this is a bunch of "rows" from Stripe
	Charges []chargeutils.Data `json:"charges"`

	//Data for each refund for the report
	//similar to Charges above
	Refunds []chargeutils.RefundData `json:"refunds"`

	//The total amount of all charges within the report date range
	TotalAmount string `json:"total_amount"`

	//Number of charges within the report date range
	NumCharges uint16 `json:"num_charges"`
}
