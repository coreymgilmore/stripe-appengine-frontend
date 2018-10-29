package card

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/company"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sessionutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/templates"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/users"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
)

//Report gets the data for charges and refunds by the defined filters (date range and customer) and builds the reports page
//The reports show up in a different page so they are easily printable and more easily inspected.
//Date range is inclusive of start and end day.
func Report(w http.ResponseWriter, r *http.Request) {
	//get form values
	datastoreID := r.FormValue("customer-id")
	startString := r.FormValue("start-date")
	endString := r.FormValue("end-date")
	hoursToUTC := r.FormValue("timezone")
	hoursToUTCInt, _ := strconv.Atoi(hoursToUTC)

	//get report data form stripe
	//make sure inputs are given
	if len(startString) == 0 {
		output.Error(errMissingInput, "You must supply a 'start-date'.", w)
		return
	}
	if len(endString) == 0 {
		output.Error(errMissingInput, "You must supply a 'end-date'.", w)
		return
	}
	if len(hoursToUTC) == 0 {
		output.Error(errMissingInput, "You must supply a 'timezone'.", w)
		return
	}

	//get timezone offset
	//adjust for the local timezone the user is in so that the date range is correct
	//hoursToUTC is a number generated by JS (-4 for EST)
	tzOffset := calcTzOffset(hoursToUTC)

	//get datetimes from provided start and end date strings
	startDt, err := time.Parse("2006-01-02 -0700", startString+" "+tzOffset)
	if err != nil {
		output.Error(err, "Could not convert start date to a time.Time datetime.", w)
		return
	}
	endDt, err := time.Parse("2006-01-02 -0700", endString+" "+tzOffset)
	if err != nil {
		output.Error(err, "Could not convert end date to a time.Time datetime.", w)
		return
	}

	//get end of day datetime
	//need to get 23:59:59 so we include the whole day
	endDt = endDt.Add((24*60-1)*time.Minute + (59 * time.Second))

	//get unix timestamps
	//stripe only accepts timestamps for filtering charges
	startUnix := startDt.Unix()
	endUnix := endDt.Unix()

	//init stripe
	c := r.Context()
	sc := createAppengineStripeClient(c)

	//get data on charges
	charges, numCharges, totalCharged, totalChargedLessFees := getListOfCharges(c, sc, r, datastoreID, startUnix, endUnix)

	//get data on refunds
	refunds, numRefunds, totalRefunded := getListOfRefunds(sc, startUnix, endUnix)

	//format dates to timezone the user is in
	// for _, chg := range charges {
	// 	chg.Da
	// }

	//get logged in user's data
	//for determining if receipt/refund buttons need to be hidden or shown based on user's access rights
	userID := sessionutils.GetUserID(r)
	userdata, _ := users.Find(c, userID)

	//store data for building template
	result := reportData{
		UserData:             userdata,
		StartDate:            startDt,
		EndDate:              endDt,
		Charges:              charges,
		Refunds:              refunds,
		TotalCharges:         totalCharged,
		TotalChargesLessFees: totalChargedLessFees,
		TotalRefunds:         totalRefunded,
		TotalRefundsLessFees: "",
		NumCharges:           numCharges,
		NumRefunds:           numRefunds,
		TimezoneOffset:       hoursToUTCInt,
	}

	//build template to display report
	//separate page in gui
	templates.Load(w, "report", result)
	return
}

//getListOfCharges gets the list of charges and returns data about them
//This filters the list of charges by date range and customer.
//The returned data includes the total amount of the charges with and without fees
//and the number of charges.
func getListOfCharges(c context.Context, sc *client.API, r *http.Request, datastoreID string, start, end int64) (data []ChargeData, numCharges uint16, total, totalLessFees string) {
	//retrieve data from stripe
	//date is a range inclusive of the days the user chose
	//limit of 100 is the max per stripe
	params := &stripe.ChargeListParams{}
	params.Filters.AddFilter("created", "gte", strconv.FormatInt(start, 10))
	params.Filters.AddFilter("created", "lte", strconv.FormatInt(end, 10))
	params.Filters.AddFilter("limit", "", "100")

	//check if we need to filter by a specific customer
	//look up stripe customer id by the datastore id
	if len(datastoreID) != 0 {
		datastoreIDInt, _ := strconv.ParseInt(datastoreID, 10, 64)
		custData, err := findByDatastoreID(c, datastoreIDInt)
		if err != nil {
			return
		}

		params.Filters.AddFilter("customer", "", custData.StripeCustomerToken)
	}

	//get list of charges
	//loop through each charge and extract charge data
	//add up total amount of all charges
	charges := sc.Charges.List(params)
	var amountTotal int64
	for charges.Next() {
		//get each charges data
		chg := charges.Charge()
		d := ExtractDataFromCharge(chg)

		//make sure this charge was captured
		//do not count charges that failed
		if d.Captured == false {
			continue
		}

		data = append(data, d)

		//increment totals
		amountTotal += d.AmountCents
		numCharges++
	}

	//calculate fees
	//fees should be returned as a dollar.cents number
	companyInfo, _ := company.Get(r)

	fixedFee := float64(numCharges) * companyInfo.FixedFee
	percentFee := float64(amountTotal) / 100 * companyInfo.PercentFee
	percentFeeRounded := round(percentFee)
	fees := fixedFee + percentFeeRounded

	//convert amounts to dollars
	amountTotalFloat := (float64(amountTotal) / 100)
	total = strconv.FormatFloat(amountTotalFloat, 'f', 2, 64)
	totalLessFees = strconv.FormatFloat(amountTotalFloat-fees, 'f', 2, 64)

	return
}

//getListOfRefunds  gets the list of refunds and returns data about them
//This filters the list of refunds by date range.
//We cannot filter by company when looking up refunds, unfortunately (Stripe issue).
//This looks up refunds by iterating through the list of events that
//happened on our Stripe account.
func getListOfRefunds(sc *client.API, start, end int64) (refunds []RefundData, numRefunds uint16, total string) {
	//retrieve refunds
	eventParams := &stripe.EventListParams{}
	eventParams.Filters.AddFilter("created", "gte", strconv.FormatInt(start, 10))
	eventParams.Filters.AddFilter("created", "lte", strconv.FormatInt(end, 10))
	eventParams.Filters.AddFilter("limit", "", "100")
	eventParams.Filters.AddFilter("type", "", "charge.refunded")

	events := sc.Events.List(eventParams)
	refunds = ExtractRefundsFromEvents(events)

	var amountTotal int64
	for _, v := range refunds {
		numRefunds++
		amountTotal += v.AmountCents
	}

	//calculate total less fees
	/*
	 * Cannot do this since we don't know if a refund was a full refund or partial refund.
	 * Fixed fees are only refunded on full refunds.
	 * We can get a list of all refunds...but what if a charge was refunded on two transaction on different days?
	 * We would need some way to check this and know which day the fixed fees were refunded on.
	 *
	 */

	//convert amount to dollars
	total = strconv.FormatFloat((float64(amountTotal) / 100), 'f', 2, 64)

	return
}

//round rounds a number to two decimal places
//this is used when calculating the percentage fees for a charge
func round(f float64) float64 {

	//the number of digits after the decimal point we want
	const sigfigs float64 = 2

	//the number we multiply by to shift the decimal point
	//shift = 100, this will result in us getting the number as cents
	var shift = 10 * sigfigs

	//shift the decimal to the right
	fShiftedRight := f * shift

	//round
	fCentsRounded := math.Floor(fShiftedRight + 0.5)

	//shift the decimal back to the left to get dollar.cents value
	return fCentsRounded / shift
}
