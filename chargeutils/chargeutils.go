package chargeutils

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/event"
)

type Data struct {
	Id            string `json:"charge_id"`       //the stripe charge id
	AmountCents   uint64 `json:"amount_cents"`    //whole number
	AmountDollars string `json:"amount_dollars"`  //without dollar sign
	Captured      bool   `json:"captured"`        //True or False
	CapturedStr   string `json:"captured_string"` //"true" or "false"
	Timestamp     string `json:"timestamp"`       //unix timestamp of the time that stripe charged the card
	Invoice       string `json:"invoice_num"`
	Po            string `json:"po_num"`
	StripeCustId  string `json:"stripe_customer_id"` //this is the id given to the customer by stripe and is used to charge the card
	Customer      string `json:"customer_name"`
	CustomerId    string `json:"customer_id"` //this is your unique id you gave the customer when you saved the card
	User          string `json:"username"`    //username of the user who charged the card
	Cardholder    string `json:"cardholder"`
	LastFour      string `json:"last4"`
	Expiration    string `json:"expiration"`
	CardBrand     string `json:"card_brand"`
}

type RefundData struct {
	Refunded      bool
	AmountCents   uint64
	AmountDollars string
	Timestamp     string
	Invoice       string
	LastFour      string
	Expiration    string
	Customer      string
	User          string
	Reason        string
}

//**********************************************************************
//EXTRA DATA FROM A CHARGE OBJECT
//stripe has some funky way of structuring the charge data so this makes it easier
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

//EXTRACT REFUND DATA
//kind of a pain in ass because Stripe returns this data as a json with no way to convert to a struct
//have to type convert each field as needed
//note: there can be many refunds for a single charge
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

		meta := charge["metadata"].(map[string]interface{})
		custName := meta["customer_name"].(string)
		invoice := meta["invoice_num"].(string)

		//get refund data
		//have to check forr "null" fields
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
