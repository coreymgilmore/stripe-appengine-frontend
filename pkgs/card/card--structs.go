package card

import (
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/chargeutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
)

//CustomerDatastore is the format for data being saved to the datastore when a new customer is added
//This data is also returned when looking up a customer.
//We use the data in this struct to process a charge for this customer.
//Each struct saved in the datastore is called an entity.  Each entity has a key that
//is used to look it up.  This key is not included in the entity (unlike an ID column in a sql row).
//Each key also has an ID portion to it.  The ID is just a shorter version and is easier to use at times.
//This ID is refered to as the "datastore ID".
type CustomerDatastore struct {
	CustomerID          string `datastore:"CustomerId" json:"customer_id"` //the CRM ID of the customer.
	CustomerName        string `json:"customer_name"`                      //the name of the customer, usually this is the company an individual card holder works for
	Cardholder          string `json:"cardholder_name"`                    //the name on the card
	CardExpiration      string `json:"card_expiration"`                    //used to just show in the gui which card is on file, MM/YYYY
	CardLast4           string `json:"card_last4"`                         //used to just show in the gui which card is on file
	StripeCustomerToken string `json:"-"`                                  //the id returned when a card is saved via Stripe, this id uniquely identifies this card for this customer
	DatetimeCreated     string `json:"-"`                                  //when was this card added to the app
	AddedByUser         string `json:"added_by"`                           //which user of the app saved the card
}

//chargeSuccessful is used to return data to the gui when a charge is processed
//This data shows which card was processed and some confirmation details.
type chargeSuccessful struct {
	CustomerName   string `json:"customer_name"`
	Cardholder     string `json:"cardholder_name"`
	CardExpiration string `json:"card_expiration"`
	CardLast4      string `json:"card_last4"`
	Amount         string `json:"amount"`    //the amount of the charge as a dollar amount string
	Invoice        string `json:"invoice"`   //the invoice number to reference for this charge
	Po             string `json:"po"`        //the po number to reference for this charge
	Datetime       string `json:"datetime"`  //when the charge was processed
	ChargeID       string `json:"charge_id"` //the unique id returned by stripe for this charge, used to show a receipt if needed or process a refund
}

//List is used to return the list of cards available to be charged to build the gui
//The list is used to build the autocomplete datalist in the UI.
type List struct {
	CustomerName string `json:"customer_name"` //company name for the card
	ID           int64  `json:"id"`            //the datastore id of the card, this is what uniquely identifies the card in the datatore so we can look data.
}

//reportData is used to build the report UI
type reportData struct {
	UserData             users.User           `json:"user_data"`              //the data for the logged in user, so we can show/hide certain UI elements based on the user's access rights.
	StartDate            time.Time            `json:"start_datetime"`         //The datetime we are filtering for getting report data to limit the days a report gets data for.
	EndDate              time.Time            `json:"end_datetime"`           // " " " "
	Charges              []chargeutils.Charge `json:"charges"`                //Data for each charge for the report, this is a bunch of "rows" from Stripe
	Refunds              []chargeutils.Refund `json:"refunds"`                //Data for each refund for the report, similar to Charges above
	TotalCharges         string               `json:"total_amount"`           //The total amount of all charges within the report date range
	TotalChargesLessFees string               `json:"total_amount_less_fees"` //Deducting fees to show what amount we will actually get from Stripe
	TotalRefunds         string               `json:"total_refund"`           //How much money we refunded.
	TotalRefundsLessFees string               `json:"total_refund_less_fees"` //How much money we will actually get back from Stripe.
	NumCharges           uint16               `json:"num_charges"`            //Number of charges within the report date range
	NumRefunds           uint16               `json:"num_refunds"`            //Same as above but for refunds
}
