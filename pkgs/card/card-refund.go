package card

import (
	"net/http"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/stripe/stripe-go/v72"
)

//Refund handles refunding a charge on a card
func Refund(w http.ResponseWriter, r *http.Request) {
	//get inputs
	chargeID := r.FormValue("chargeId")
	amount := r.FormValue("amount")
	reason := r.FormValue("reason")

	//make sure inputs were given
	if len(chargeID) == 0 {
		output.Error(errMissingInput, "A charge ID was not provided. This is a serious error. Please contact an administrator.", w)
		return
	}
	if len(amount) == 0 {
		output.Error(errMissingInput, "No amount was given to refund.", w)
		return
	}

	//convert refund amount to cents
	//stripe requires amount in a whole number
	amountCents, err := getAmountAsIntCents(amount)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w)
		return
	}

	//get username of logged in user
	//for tracking who processed this refund
	username := sessionutils.GetUsername(r)

	//build refund
	params := &stripe.RefundParams{
		Charge: stripe.String(chargeID),
		Amount: stripe.Int64(int64(amountCents)),
	}

	//add metadata to refund
	//same field name as when creating a charge
	params.AddMetadata("processed_by", username)

	//get reason code for refund
	//these are defined by stripe
	switch reason {
	case "duplicate":
		params.Reason = stripe.String(string(stripe.RefundReasonDuplicate))
	case "requested_by_customer":
		params.Reason = stripe.String(string(stripe.RefundReasonRequestedByCustomer))
	case "fraudulent":
		params.Reason = stripe.String(string(stripe.RefundReasonFraudulent))
	}

	//init stripe
	c := r.Context()
	sc := CreateStripeClient(c)

	//create refund with stripe
	_, err = sc.Refunds.New(params)
	if err != nil {
		stripeErr := err.(*stripe.Error)
		stripeErrMsg := stripeErr.Msg
		output.Error(errStripe, stripeErrMsg, w)
		return
	}

	//done
	output.Success("refund-done", nil, w)
}
