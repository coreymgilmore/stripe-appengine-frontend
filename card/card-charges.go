/*
	This file is part of the card package.
	This specifically deals with processing charges (and refunding charges).
	These functions are broken out into a separate file for organizational purposes.
*/

package card

import (
	"net/http"
	"strconv"

	"google.golang.org/appengine"

	"github.com/coreymgilmore/timestamps"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/refund"

	"output"
	"memcacheutils"
	"sessionutils"
)

//CHARGE A CARD
func Charge(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreId := r.FormValue("datastoreId")
	customerName := r.FormValue("customerName")
	amount := r.FormValue("amount")
	invoice := r.FormValue("invoice")
	poNum := r.FormValue("po")

	//validation
	if len(datastoreId) == 0 {
		output.Error(ErrMissingInput, "A customer ID should have been submitted automatically but was not. Please contact an administrator.", w)
		return
	}
	if len(amount) == 0 {
		output.Error(ErrMissingInput, "No amount was provided. You cannot charge a card nothing!", w)
		return
	}

	//get amount as cents
	amountCents, err := getAmountAsIntCents(amount)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w)
		return
	}

	//check if amount is greater than the minimum charge
	//min charge may be greater than 0 because of transactions costs
	//for example, stripe takes 30 cents...it does not make sense to charge a card for < 30 cents
	if amountCents < MIN_CHARGE {
		output.Error(ErrChargeAmountTooLow, "You must charge at least "+strconv.FormatInt(MIN_CHARGE, 10)+" cents.", w)
		return
	}

	//look up stripe customer id from datastore
	c := appengine.NewContext(r)
	datastoreIdInt, _ := strconv.ParseInt(datastoreId, 10, 64)
	custData, err := findByDatastoreId(c, datastoreIdInt)
	if err != nil {
		output.Error(err, "An error occured while looking up the customer's Stripe information.", w)
		return
	}

	//make sure customer name matches
	//just another catch in case of strange errors and mismatched data
	if customerName != custData.CustomerName {
		output.Error(err, "The customer name did not match the data for the customer ID. Please log out and try again.", w)
		return
	}

	//get username of logged in user
	//used for tracking who processed a charge
	//for audits and reports
	session := sessionutils.Get(r)
	username := session.Values["username"].(string)

	//init stripe
	sc := createAppengineStripeClient(c)

	//build charge object
	chargeParams := &stripe.ChargeParams{
		Customer:  custData.StripeCustomerToken,
		Amount:    amountCents,
		Currency:  CURRENCY,
		Desc:      "Charge for invoice: " + invoice + ", purchase order: " + poNum + ".",
		Statement: formatStatementDescriptor(),
	}

	//add metadata to charge
	//used for reports and receipts
	chargeParams.AddMeta("customer_name", customerName)
	chargeParams.AddMeta("datastore_id", datastoreId)
	chargeParams.AddMeta("customer_id", custData.CustomerId)
	chargeParams.AddMeta("invoice_num", invoice)
	chargeParams.AddMeta("po_num", poNum)
	chargeParams.AddMeta("charged_by", username)

	//process the charge
	chg, err := sc.Charges.New(chargeParams)
	if err != nil {
		stripeErr := err.(*stripe.Error)
		stripeErrMsg := stripeErr.Msg
		output.Error(ErrStripe, stripeErrMsg, w)
		return
	}

	//charge successful
	//save charge to memcache
	//less data to get from stripe if receipt is needed
	memcacheutils.Save(c, chg.ID, chg)

	//return to client
	//build struct to output a success message to the client
	out := chargeSuccessful{
		CustomerName:   customerName,
		Cardholder:     custData.Cardholder,
		CardExpiration: custData.CardExpiration,
		CardLast4:      custData.CardLast4,
		Amount:         amount,
		Invoice:        invoice,
		Po:             poNum,
		Datetime:       timestamps.ISO8601(),
		ChargeId:       chg.ID,
	}
	output.Success("cardCharged", out, w)
	return
}

//REFUND A CHARGE
func Refund(w http.ResponseWriter, r *http.Request) {
	//get form values
	chargeId := r.FormValue("chargeId")
	amount := r.FormValue("amount")
	reason := r.FormValue("reason")

	//make sure inputs were given
	if len(chargeId) == 0 {
		output.Error(ErrMissingInput, "A charge ID was not provided. This is a serious error. Please contact an administrator.", w)
		return
	}
	if len(amount) == 0 {
		output.Error(ErrMissingInput, "No amount was given to refund.", w)
		return
	}

	//convert refund amount to cents
	//stripe requires cents
	amountCents, err := getAmountAsIntCents(amount)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w)
		return
	}

	//get username of logged in user
	//for tracking who processed this refund
	session := sessionutils.Get(r)
	username := session.Values["username"].(string)

	//build refund
	params := &stripe.RefundParams{
		Charge: chargeId,
		Amount: amountCents,
	}

	//add metadata to refund
	//same field name as when creating a charge
	params.AddMeta("charged_by", username)

	//get reason code for refund
	if reason == "duplicate" {
		params.Reason = refund.RefundDuplicate
	} else if reason == "requested_by_customer" {
		params.Reason = refund.RefundRequestedByCustomer
	}

	//init stripe
	c := appengine.NewContext(r)
	sc := createAppengineStripeClient(c)

	//create refund with stripe
	_, err = sc.Refunds.New(params)
	if err != nil {
		stripeErr := err.(*stripe.Error)
		stripeErrMsg := stripeErr.Msg
		output.Error(ErrStripe, stripeErrMsg, w)
		return
	}

	//done
	output.Success("refund-done", nil, w)
	return
}