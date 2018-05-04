/*
Package chargeutils impletments some tooling to pull data out of Stripe api calls
that is used to build reports.

Data is retrieved/returned from Stripe in a very "busy" format.  It has lots of extra
data.  This cleans up the data for us to use in building reports.
*/
package chargeutils

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/event"
)

//Charge is the format in which we return data that is part of a Stripe charge object
//stripe returns a bunch of data when a charge is made (or when looking up a charge by the charge id)
//this is the data we need from that charge object
type Charge struct {
	ID            string `json:"charge_id,omitempty"`          //the stripe charge id
	AmountCents   uint64 `json:"amount_cents,omitempty"`       //the amount of the charge in cents
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
}

//Refund is the format in which we return data that is part of a refund
//this makes it easier to deal with the funky way stripe returns refund data
type Refund struct {
	Refunded      bool   //was this a refund, should always be true
	AmountCents   uint64 //the amount of the refund in cents, this amount can be less than or equal to the corresponding charge
	AmountDollars string //amount of the refund in dollars (without $ symbol)
	Timestamp     string //unix timestamp of the time that stripe refunded the card
	Invoice       string //metadata field with extra info on the charge
	LastFour      string //used to identify the card when looking at a report
	Expiration    string //" " " "
	Customer      string //name of the customer from the app engine datastore, name of the customer we charged
	User          string //username of the user who refunded the card
	Reason        string //why was the card refunded, this is a special value dictated by stripe
}

//ExtractDataFromCharge pulls out the fields of data we want from a stripe charge object
//we only need certain info from the stripe charge object, this pulls the needed fields out
//also does some formating for using the data in the gui
func ExtractDataFromCharge(chg *stripe.Charge) (data Charge) {
	//charge info
	id := chg.ID
	amountInt := chg.Amount
	captured := chg.Captured
	capturedStr := strconv.FormatBool(captured)
	timestamp := chg.Created

	//skip the rest of this if captured is false
	//this means the charge was not processed
	//for example: the card was declined
	if captured == false {
		return
	}

	//metadata
	meta := chg.Meta
	customerName := meta["customer_name"]
	customerID := meta["customer_id"]
	invoice := meta["invoice_num"]
	po := meta["po_num"]
	username := meta["processed_by"]

	autoCharge, _ := strconv.ParseBool(meta["auto_charge"])
	autoChargeReferrer := meta["auto_charge_referrer"]
	autoChargeReason := meta["auto_charge_reason"]

	//customer info
	customer := chg.Customer
	j, _ := json.Marshal(customer)
	customer.UnmarshalJSON(j)
	stripeCustID := customer.ID

	//card info
	source := chg.Source
	j2, _ := json.Marshal(source)
	source.UnmarshalJSON(j2)
	card := source.Card
	cardholder := card.Name
	expMonth := strconv.FormatInt(int64(card.Month), 10)
	expYear := strconv.FormatInt(int64(card.Year), 10)
	exp := expMonth + "/" + expYear
	last4 := card.LastFour
	cardBrand := string(card.Brand)

	//convert amount to dollars
	amountDollars := strconv.FormatFloat((float64(amountInt) / 100), 'f', 2, 64)

	//convert timetamp to datetime
	datetime := time.Unix(timestamp, 0).Format("2006-01-02T15:04:05.000Z")

	//build data struct to return
	data = Charge{
		ID:            id,
		AmountCents:   amountInt,
		AmountDollars: amountDollars,
		Captured:      captured,
		CapturedStr:   capturedStr,
		Timestamp:     datetime,
		Invoice:       invoice,
		Po:            po,
		StripeCustID:  stripeCustID,
		Customer:      customerName,
		CustomerID:    customerID,
		User:          username,
		Cardholder:    cardholder,
		LastFour:      last4,
		Expiration:    exp,
		CardBrand:     cardBrand,

		AutoCharge:         autoCharge,
		AutoChargeReferrer: autoChargeReferrer,
		AutoChargeReason:   autoChargeReason,
	}

	return
}

//ExtractRefundsFromEvents pulls out the data for each refund from the list of events and formats the data as needed
//stripe does not allow easy retrieving of refund data like charge data
//have to search in the "history" aka event log for refunds
//then have to parse the data since the data is in json format and there is no easy way to convert the data to a struct
//note: there can be many refunds for a single charge (that total less than or equal to the total amount charged)
func ExtractRefundsFromEvents(eventList *event.Iter) (r []Refund) {
	//loop through each refund event
	//each event is a charge
	//each charge can have one or more refunds
	for eventList.Next() {
		event := eventList.Event()
		charge := event.Data.Obj

		//get charge data
		//*must* use map[string]interface and then type assert to .(string)...go throws errors otherwise
		card := charge["source"].(map[string]interface{})
		lastFour := card["last4"].(string)
		expMonth := strconv.FormatInt(int64(card["exp_month"].(float64)), 10)
		expYear := strconv.FormatInt(int64(card["exp_year"].(float64)), 10)
		expiration := expMonth + "/" + expYear

		//charge meta data
		meta := charge["metadata"].(map[string]interface{})
		custName := meta["customer_name"].(string)
		invoice := meta["invoice_num"].(string)

		//get refund data
		//have to check for "null" fields
		refundData := charge["refunds"].(map[string]interface{})
		refundList := refundData["data"].([]interface{})
		for _, v := range refundList {
			refund := v.(map[string]interface{})
			refundedAmountInt := refund["amount"].(float64)
			refundedTimestamp := refund["created"].(float64)

			refundReason := "unknown"
			if refund["reason"] != nil {
				refundReason = refund["reason"].(string)
			}

			refundedBy := "unknown"
			if refund["metadata"] != nil {
				rdMeta := refund["metadata"].(map[string]interface{})
				if rdMeta["processed_by"] != nil {
					refundedBy = rdMeta["processed_by"].(string)
				}
			}

			//get refunded amount in dollars
			refundedDollars := strconv.FormatFloat((refundedAmountInt / 100), 'f', 2, 64)

			//convert timestamp to datetime
			datetime := time.Unix(int64(refundedTimestamp), 0).Format("2006-01-02T15:04:05.000Z")

			//build struct to build template with
			rr := Refund{
				Refunded:      true,
				AmountCents:   uint64(refundedAmountInt),
				AmountDollars: refundedDollars,
				Timestamp:     datetime,
				Invoice:       invoice,
				LastFour:      lastFour,
				Expiration:    expiration,
				Customer:      custName,
				User:          refundedBy,
				Reason:        refundReason,
			}

			r = append(r, rr)
		}
	}

	//return list of refunds
	return
}
