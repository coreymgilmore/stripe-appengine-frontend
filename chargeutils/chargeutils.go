/*
Package chargeutils implements tools to pull data out of Stripe api call responses and use it to display information in the app's ui.

Data returned from Stripe has some funky formatting. It is important to extract the data points we need and format the data
in a better way to that it can be used in the gui.
*/

package chargeutils

import (
	"encoding/json"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/event"
	"strconv"
	"time"
)

//Data is the format in which we return data that is part of a Stripe charge object
//stripe returns a bunch of data when a charge is made (or when looking up a charge by the charge id)
//this is the data we need from that charge object
type Data struct {
	//the stripe charge id
	Id string `json:"charge_id"`

	//the amount of the charge in cents
	AmountCents uint64 `json:"amount_cents"`

	//amount of the charge in dollars (without $ symbol)
	AmountDollars string `json:"amount_dollars"`

	//determines if the charge was successfully placed on a real credit card
	Captured    bool   `json:"captured"`        //True or False
	CapturedStr string `json:"captured_string"` //"true" or "false"

	//unix timestamp of the time that stripe charged the card
	Timestamp string `json:"timestamp"`

	//metadata fields with extra info on the charge
	//the user provides this data when a charge is processed
	Invoice string `json:"invoice_num"`
	Po      string `json:"po_num"`

	//this is the id given to the customer by stripe and is used to charge the card
	StripeCustId string `json:"stripe_customer_id"`

	//name of the customer from the app engine datastore
	//the name of the company a card belongs to
	Customer string `json:"customer_name"`

	//this is your unique id you gave the customer when you saved the card
	//from a crm
	CustomerId string `json:"customer_id"`

	//username of the user who charged the card
	User string `json:"username"`

	//name on the card
	Cardholder string `json:"cardholder"`

	//used to identify the card when looking at the receipt or in a report
	LastFour   string `json:"last4"`
	Expiration string `json:"expiration"`
	CardBrand  string `json:"card_brand"`
}

//RefundData is the format in which we return data that is part of a refund
//this makes it easier to deal with the funky way stripe returns refund data
type RefundData struct {
	//was this a refund
	//should always be true
	Refunded bool

	//the amount of the refund in cents
	//this amount can be less than or equal to the corresponding charge
	//cannot refund more than was originally charged
	AmountCents uint64

	//amount of the refund in dollars (without $ symbol)
	AmountDollars string

	//unix timestamp of the time that stripe refunded the card
	Timestamp string

	//metadata field with extra info on the charge
	Invoice string

	//used to identify the card when looking at a report
	LastFour   string
	Expiration string

	//name of the customer from the app engine datastore
	//the name of the company a card belongs to
	Customer string

	//username of the user who refunded the card
	User string

	//why was the card refunded
	//this has special values dictated by stripe
	Reason string
}

//ExtractData pulls out the fields of data we want from a stripe charge object
//we only need certain info from the stripe charge object, this pulls the needed fields out
//also does some formating for using the data in the gui
func ExtractData(chg *stripe.Charge) Data {
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
		return Data{}
	}

	//metadata
	meta := chg.Meta
	customerName := meta["customer_name"]
	customerId := meta["customer_id"]
	invoice := meta["invoice_num"]
	po := meta["po_num"]
	username := meta["charged_by"]

	//customer info
	customer := chg.Customer
	j, _ := json.Marshal(customer)
	customer.UnmarshalJSON(j)
	stripeCustId := customer.ID

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
	d := Data{
		Id:            id,
		AmountCents:   amountInt,
		AmountDollars: amountDollars,
		Captured:      captured,
		CapturedStr:   capturedStr,
		Timestamp:     datetime,
		Invoice:       invoice,
		Po:            po,
		StripeCustId:  stripeCustId,
		Customer:      customerName,
		CustomerId:    customerId,
		User:          username,
		Cardholder:    cardholder,
		LastFour:      last4,
		Expiration:    exp,
		CardBrand:     cardBrand,
	}

	return d
}

//ExtractRefunds pulls out the data for each refund from the list of events and formats the data as needed
//stripe does not allow easy retrieving of refund data like charge data
//have to search in the "history" aka event log for refunds
//then have to parse the data since the data is in json format and there is no easy way to convert the data to a struct
//note: there can be many refunds for a single charge (that total less than or equal to the total amount charged)
func ExtractRefunds(eventList *event.Iter) []RefundData {
	//placeholder for returning data
	output := make([]RefundData, 0, 10)

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
				if rdMeta["charged_by"] != nil {
					refundedBy = rdMeta["charged_by"].(string)
				}
			}

			//get refunded amount in dollars
			refundedDollars := strconv.FormatFloat((refundedAmountInt / 100), 'f', 2, 64)

			//convert timestamp to datetime
			datetime := time.Unix(int64(refundedTimestamp), 0).Format("2006-01-02T15:04:05.000Z")

			//build struct to build template with
			x := RefundData{
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

			output = append(output, x)
		}
	}

	//return list of refunds
	return output
}
