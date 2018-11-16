package card

import (
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
)

//CustomerDatastore is the data stored in the db (Google Cloud Datatore) about a customer.
//This information is added when a new customer/card is added.  We use this data to process
//charges for a customer by linking our CRM id or customer name to Stripe's id for the card.
//In the Google Cloud Datastore each customer is saved as an entity (think sql row).  Each entity
//has a key that is used to look it up.  This key is not included in the entity unlike an ID column
//in a sql row.  This key has a subcomponent called anID.  This is referred to as the "datastore ID".
type CustomerDatastore struct {
	CustomerID          string `datastore:"CustomerId" json:"customer_id"` //the CRM ID of the customer.
	CustomerName        string `json:"customer_name"`                      //the name of the customer, usually this is the company an individual card holder works for
	Cardholder          string `json:"cardholder_name"`                    //the name on the card
	CardExpiration      string `json:"card_expiration"`                    //used to just show in the gui which card is on file, MM/YYYY
	CardLast4           string `json:"card_last4"`                         //used to just show in the gui which card is on file
	StripeCustomerToken string `json:"-"`                                  //the id returned when a card is saved via Stripe, this id uniquely identifies this card for this customer
	DatetimeCreated     string `json:"-"`                                  //when was this card added to the app
	AddedByUser         string `json:"added_by"`                           //which user of the app saved the card

	//fields not used in cloud datastore
	ID int64 `json:"sqlite_user_id"`
}

//chargeSuccessful is used to return data to the gui when a charge is processed
//This data shows which card was processed and some confirmation details.
type chargeSuccessful struct {
	CustomerName   string `json:"customer_name"`
	Cardholder     string `json:"cardholder_name"`
	CardExpiration string `json:"card_expiration"`
	CardLast4      string `json:"card_last4"`
	Amount         string `json:"amount"`          //the amount of the charge as a dollar amount string
	Invoice        string `json:"invoice"`         //the invoice number to reference for this charge
	Po             string `json:"po"`              //the po number to reference for this charge
	Datetime       string `json:"datetime"`        //when the charge was processed
	ChargeID       string `json:"charge_id"`       //the unique id returned by stripe for this charge, used to show a receipt if needed or process a refund
	AuthorizedOnly bool   `json:"authorized_only"` //true if charge was authorized but not charged
}

//List is used to return the list of cards available to be charged to build the gui
//The list is used to build the autocomplete datalist in the UI.
type List struct {
	CustomerName string `json:"customer_name"` //company name for the card
	ID           int64  `json:"id"`            //the datastore id of the card, this is what uniquely identifies the card in the datatore so we can look data.
}

//ChargeData is the data from a charge that we use to build the gui
//Stripe's charge struct (stripe.Charge) returns a lot more info than we need so we don't use it.
type ChargeData struct {
	ID            string `json:"charge_id,omitempty"`          //the stripe charge id
	AmountCents   int64  `json:"amount_cents,omitempty"`       //the amount of the charge in cents
	AmountDollars string `json:"amount_dollars,omitempty"`     //amount of the charge in dollars (without $ symbol)
	Captured      bool   `json:"captured,omitempty"`           //determines if the charge was successfully placed on a real credit card
	CapturedStr   string `json:"captured_string,omitempty"`    //see above
	Timestamp     string `json:"timestamp,omitempty"`          //unix timestamp of the time that stripe charged the card
	Invoice       string `json:"invoice_num,omitempty"`        //some extra info that was provided when the user processed the charge
	Po            string `json:"po_num,omitempty"`             // " " " "
	StripeCustID  string `json:"stripe_customer_id,omitempty"` //this is the id given to the customer by stripe and is used to charge the card
	Customer      string `json:"customer_name,omitempty"`      //name of the customer from the app engine datastore, the name of the company a card belongs to
	CustomerID    string `json:"customer_id,omitempty"`        //the unique id you gave the customer when you saved the card, from a CRM
	User          string `json:"username,omitempty"`           //username of the user who charged the card
	Cardholder    string `json:"cardholder,omitempty"`         //name on the card
	LastFour      string `json:"last4,omitempty"`              //used to identify the card when looking at the receipt or in a report
	Expiration    string `json:"expiration,omitempty"`         // " " " "
	CardBrand     string `json:"card_brand,omitempty"`         // " " " "

	//data for automatically completed charges (api request charges)
	AutoCharge         bool   `json:"auto_charge,omitempty"`          //true if we made this charge automatically through api request
	AutoChargeReferrer string `json:"auto_charge_referrer,omitempty"` //the name of the app that requested the charge
	AutoChargeReason   string `json:"auto_charge_reason,omitempty"`   //if one app/referrer will place charges for many reasons, detail that reason here; so we know what process/func caused the charge

	//data about an authed & captured charge
	AuthorizedByUser   string `json:"authorized_by_user"`
	AuthorizedDatetime string `json:"authorized_datetime"`
	ProcessedDatetime  string `json:"processed_datetime"`
}

//RefundData is the data from a refund that we use the build the gui
//Stripe returns more info thatn we need so we use our own struct to organize the data better
type RefundData struct {
	Refunded      bool   //was this a refund, should always be true
	AmountCents   int64  //the amount of the refund in cents, this amount can be less than or equal to the corresponding charge
	AmountDollars string //amount of the refund in dollars (without $ symbol)
	Timestamp     string //unix timestamp of the time that stripe refunded the card
	Invoice       string //metadata field with extra info on the charge
	LastFour      string //used to identify the card when looking at a report
	Expiration    string //" " " "
	Customer      string //name of the customer from the app engine datastore, name of the customer we charged
	User          string //username of the user who refunded the card
	Reason        string //why was the card refunded, this is a special value dictated by stripe
}

//reportData is used to build the report UI
type reportData struct {
	UserData             users.User   `json:"user_data"`              //the data for the logged in user, so we can show/hide certain UI elements based on the user's access rights.
	StartDate            time.Time    `json:"start_datetime"`         //The datetime we are filtering for getting report data to limit the days a report gets data for.
	EndDate              time.Time    `json:"end_datetime"`           // " " " "
	Charges              []ChargeData `json:"charges"`                //Data for each charge for the report, this is a bunch of "rows" from Stripe
	Refunds              []RefundData `json:"refunds"`                //Data for each refund for the report, similar to Charges above
	TotalCharges         string       `json:"total_amount"`           //The total amount of all charges within the report date range
	TotalChargesLessFees string       `json:"total_amount_less_fees"` //Deducting fees to show what amount we will actually get from Stripe
	TotalRefunds         string       `json:"total_refund"`           //How much money we refunded.
	TotalRefundsLessFees string       `json:"total_refund_less_fees"` //How much money we will actually get back from Stripe.
	NumCharges           uint16       `json:"num_charges"`            //Number of charges within the report date range
	NumRefunds           uint16       `json:"num_refunds"`            //Same as above but for refunds
	ReportGUITimezone    string       `json:"reprot_gui_timezone"`    //this is the timezone used to format the timestamps on the report
}
