package chargeutils 

import (
	"encoding/json"
	"time"
	"strconv"

	"github.com/stripe/stripe-go"
)

type Data struct{
	Id 				string 		`json:"charge_id"` 			//the stripe charge id
	AmountCents 	uint64 		`json:"amount_cents"` 		//whole number
	AmountDollars	string 		`json:"amount_dollars"` 	//without dollar sign
	Captured 		bool 		`json:"captured"` 			//True or False
	CapturedStr 	string 		`json:"captured_string"` 	//"true" or "false"
	Timestamp 		string 		`json:"timestamp"` 			//unix timestamp of the time that stripe charged the card
	Invoice 		string 		`json:"invoice_num"`
	Po 				string 		`json:"po_num"`
	StripeCustId 	string 		`json:"stripe_customer_id"` //this is the id given to the customer by stripe and is used to charge the card
	Customer 		string 		`json:"customer_name"`
	CustomerId 		string 		`json:"customer_id"` 		//this is your unique id you gave the customer when you saved the card
	User 			string 		`json:"username"` 			//username of the user who charged the card
	Cardholder 		string 		`json:"cardholder"`
	LastFour 		string 		`json:"last4"`
	Expiration 		string 		`json:"expiration"`
	CardBrand 		string 		`json:"card_brand"`
}

//**********************************************************************
//EXTRA DATA FROM A CHARGE OBJECT
//stripe has some funky way of structuring the charge data so this makes it easier
func ExtractData(chg *stripe.Charge) Data {
	//charge info
	id := 				chg.ID 
	amountInt := 		chg.Amount
	captured := 		chg.Captured
	capturedStr := 		strconv.FormatBool(captured)
	timestamp := 		chg.Created

	//metadata
	meta := 			chg.Meta
	customerName := 	meta["customer_name"]
	customerId := 		meta["customer_id"]
	invoice := 			meta["invoice_num"]
	po := 				meta["po_num"]
	username := 		meta["charged_by"]

	//customer info
	customer := 		chg.Customer
	j, _ := 			json.Marshal(customer)
	customer.UnmarshalJSON(j)
	stripeCustId := 	customer.ID

	//card info
	source := 			chg.Source
	j2, _ := 			json.Marshal(source)
	source.UnmarshalJSON(j2)
	card := 			source.Card
	cardholder := 		card.Name 
	expMonth := 		strconv.FormatInt(int64(card.Month), 10)
	expYear := 			strconv.FormatInt(int64(card.Year), 10)
	exp := 				expMonth + "/" + expYear
	last4 := 			card.LastFour
	cardBrand := 		string(card.Brand)

	//convert amount to dollars
	amountDollars := 	strconv.FormatFloat((float64(amountInt) / 100), 'f', 2, 64)

	//convert timetamp to datetime
	datetime := 		time.Unix(timestamp, 0).Format("2006-01-02T15:04:05.000Z")

	//build data struct to return
	d := Data{
		Id: 			id,
		AmountCents: 	amountInt,
		AmountDollars: 	amountDollars,
		Captured: 		captured,
		CapturedStr: 	capturedStr,
		Timestamp: 		datetime,
		Invoice: 		invoice,
		Po: 			po,
		StripeCustId: 	stripeCustId,
		Customer: 		customerName,
		CustomerId: 	customerId,
		User: 			username,
		Cardholder: 	cardholder,
		LastFour: 		last4,
		Expiration: 	exp,
		CardBrand: 		cardBrand,
	}

	return d
}