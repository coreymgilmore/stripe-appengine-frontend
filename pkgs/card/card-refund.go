package card

import (
	"net/http"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/refund"
)

//Refund handles refunding a charge on a card
func Refund(w http.ResponseWriter, r *http.Request) {
	//get inputs
	chargeID := r.FormValue("chargeId")
	amount := r.FormValue("amount")
	reason := r.FormValue("reason")

	//make sure inputs were given
	if len(chargeID) == 0 {
		output.Error(errMissingInput, "A charge ID was not provided. This is a serious error. Please contact an administrator.", w, r)
		return
	}
	if len(amount) == 0 {
		output.Error(errMissingInput, "No amount was given to refund.", w, r)
		return
	}

	//convert refund amount to cents
	//stripe requires amount in a whole number
	amountCents, err := getAmountAsIntCents(amount)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w, r)
		return
	}

	//get username of logged in user
	//for tracking who processed this refund
	username := sessionutils.GetUsername(r)

	//build refund
	params := &stripe.RefundParams{
		Charge: chargeID,
		Amount: amountCents,
	}

	//add metadata to refund
	//same field name as when creating a charge
	params.AddMeta("processed_by", username)

	//get reason code for refund
	//these are defined by stripe
	switch reason {
	case "duplicate":
		params.Reason = refund.RefundDuplicate
	case "requested_by_customer":
		params.Reason = refund.RefundDuplicate
	case "fraudulent":
		params.Reason = refund.RefundFraudulent
	}

	//init stripe
	c := r.Context()
	sc := createAppengineStripeClient(c)

	//create refund with stripe
	_, err = sc.Refunds.New(params)
	if err != nil {
		stripeErr := err.(*stripe.Error)
		stripeErrMsg := stripeErr.Msg
		output.Error(errStripe, stripeErrMsg, w, r)
		return
	}

	//done
	output.Success("refund-done", nil, w)
	return
}
