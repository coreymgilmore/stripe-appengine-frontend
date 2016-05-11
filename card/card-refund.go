/*
File card-refund.go implements functionality for refunding money back to a credit card.
*/

package card

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/refund"
	"google.golang.org/appengine"
	"net/http"
	"output"
	"sessionutils"
)

//REFUND A CHARGE
func Refund(w http.ResponseWriter, r *http.Request) {
	//get form values
	chargeId := r.FormValue("chargeId")
	amount := r.FormValue("amount")
	reason := r.FormValue("reason")

	//make sure inputs were given
	if len(chargeId) == 0 {
		output.Error(ErrMissingInput, "A charge ID was not provided. This is a serious error. Please contact an administrator.", w, r)
		return
	}
	if len(amount) == 0 {
		output.Error(ErrMissingInput, "No amount was given to refund.", w, r)
		return
	}

	//convert refund amount to cents
	//stripe requires cents
	amountCents, err := getAmountAsIntCents(amount)
	if err != nil {
		output.Error(err, "An error occured while converting the amount to charge into cents. Please try again or contact an administrator.", w, r)
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
	//these are defined by stripe
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
		output.Error(ErrStripe, stripeErrMsg, w, r)
		return
	}

	//done
	output.Success("refund-done", nil, w)
	return
}
