/*
Package company implements functions for handling your company's information.  This is
for the company name and address, the amount of fees you pay per transaction to
Stripe, and the text that is displayed on the credit card statement.

Company data is anything for the company: address, contact info, receipt,
statement description, and fees.
*/
package company

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/sqliteutils"

	"cloud.google.com/go/datastore"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/datastoreutils"
	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/output"
)

//datastoreKeyName is the name of the entity we save company settings under
//we only store one entity for the company settings so we use this key to always refer to it
const datastoreKeyName = "companyInfoKey"

//maxStatementDescriptorLength is the maximum length of the statement description
//this is dictated by Stripe
const maxStatementDescriptorLength = 22

//default stripe fees
const (
	defaultPercentFee = 0.0290 //percentage
	defaultFixedFee   = 0.30   //cents
)

//ErrCompanyDataDoesNotExist is thrown when no company data has been set yet
//this occurs when an admin did not go into the settings and provide the company info
var ErrCompanyDataDoesNotExist = errors.New("company: info does not exist")

//Info is the used for setting or getting the company data from the datastore
type Info struct {
	CompanyName         string  `json:"company_name"`         //for receipts
	Street              string  `json:"street"`               // " "
	Suite               string  `json:"suite"`                // " "
	City                string  `json:"city"`                 // " "
	State               string  `json:"state"`                // " "
	PostalCode          string  `json:"postal_code"`          // " "
	Country             string  `json:"country"`              // " "
	PhoneNum            string  `json:"phone_num"`            // " "
	Email               string  `json:"email"`                // " "
	PercentFee          float64 `json:"percentage_fee"`       //default is 2.90% transaction per Stripe
	FixedFee            float64 `json:"fixed_fee"`            //default is $0.30 per transaction per Stripe
	StatementDescriptor string  `json:"statement_descriptor"` //what is displayed on the statement with our charge

	//fields not used in cloud datastore
	ID int64 `json:"sqlite_user_id"`
}

//defaultCompanyInfo is the minimal amount of info required
var defaultCompanyInfo = Info{
	CompanyName:         "",
	Street:              "",
	Suite:               "",
	City:                "",
	State:               "",
	PostalCode:          "",
	Country:             "",
	PhoneNum:            "",
	Email:               "",
	PercentFee:          defaultPercentFee,
	FixedFee:            defaultFixedFee,
	StatementDescriptor: "",
}

//GetAPI is used when viewing the data in the gui or on a receipt
func GetAPI(w http.ResponseWriter, r *http.Request) {
	//get info
	info, err := Get(r)
	if err != nil {
		output.Error(err, "", w)
		return
	}

	output.Success("dataFound", info, w)
	return
}

//Get actually retrienves the information from the datastore
//putting this into a separate func cleans up code elsewhere
func Get(r *http.Request) (Info, error) {
	//placeholder
	data := Info{}
	var err error

	//use correct db
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		q := `
			SELECT *
			FROM ` + sqliteutils.TableCompanyInfo + ` 
			WHERE ID=?
		`
		err = c.Get(&data, q, sqliteutils.DefaultCompanyInfoID)

	} else {
		//connect to datastore
		c := r.Context()
		client, err := datastoreutils.Connect(c)
		if err != nil {
			return data, err
		}

		//get from datastore
		key := datastore.NameKey(datastoreutils.EntityCompanyInfo, datastoreKeyName, nil)

		//get data
		err = client.Get(c, key, &data)
		if err == datastore.ErrNoSuchEntity {
			//no company info exists yet
			//return default values
			log.Println("company.Get", "Company info doesn't exist yet.  Returning default values.")
			return defaultCompanyInfo, nil
		}
	}

	//return data found
	return data, err
}

//SaveAPI saves new or updates existing company info in the datastore
func SaveAPI(w http.ResponseWriter, r *http.Request) {
	//get form values
	name := strings.TrimSpace(r.FormValue("name"))
	street := strings.TrimSpace(r.FormValue("street"))
	suite := strings.TrimSpace(r.FormValue("suite"))
	city := strings.TrimSpace(r.FormValue("city"))
	state := strings.TrimSpace(r.FormValue("state"))
	postal := strings.TrimSpace(r.FormValue("postal"))
	country := strings.TrimSpace(r.FormValue("country"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	email := strings.TrimSpace(r.FormValue("email"))
	percentFee, _ := strconv.ParseFloat(r.FormValue("percentFee"), 64)
	fixedFee, _ := strconv.ParseFloat(r.FormValue("fixedFee"), 64)
	statementDesc := strings.TrimSpace(r.FormValue("descriptor"))

	//shorten up statement descriptor if needed
	if len(statementDesc) > maxStatementDescriptorLength {
		statementDesc = statementDesc[:maxStatementDescriptorLength]
	}

	//save the percentage fee as a decimal number with up to 4 decimal places
	//2.85% = 0.0285
	percentFeeStr := "0.0" + strconv.FormatFloat(percentFee*100, 'f', 0, 64)
	percentFee, _ = strconv.ParseFloat(percentFeeStr, 64)

	//build entity to save
	//or update existing entity
	data := Info{}
	data.CompanyName = name
	data.Street = street
	data.Suite = suite
	data.City = city
	data.State = strings.ToUpper(state)
	data.PostalCode = postal
	data.Country = strings.ToUpper(country)
	data.PhoneNum = phone
	data.Email = email
	data.PercentFee = percentFee
	data.FixedFee = fixedFee
	data.StatementDescriptor = statementDesc

	//save company info
	c := r.Context()
	err := save(c, data)
	if err != nil {
		output.Error(err, "", w)
		return
	}

	//done
	output.Success("dataSaved", data, w)
	return
}

//save does the actual saving to the datastore
func save(c context.Context, i Info) error {
	//use correct db
	if sqliteutils.Config.UseSQLite {
		c := sqliteutils.Connection
		q := `
			UPDATE ` + sqliteutils.TableCompanyInfo + ` SET 
				CompanyName=?,
				Street=?,
				Suite=?,
				City=?,
				State=?,
				PostalCode=?,
				Country=?,
				PhoneNum=?,
				Email=?,
				PercentFee=?,
				FixedFee=?,
				StatementDescriptor=?
			WHERE ID = ?
		`
		stmt, err := c.Prepare(q)
		if err != nil {
			return err
		}

		_, err = stmt.Exec(
			i.CompanyName,
			i.Street,
			i.Suite,
			i.City,
			i.State,
			i.PostalCode,
			i.Country,
			i.PhoneNum,
			i.Email,
			i.PercentFee,
			i.FixedFee,
			i.StatementDescriptor,
			sqliteutils.DefaultCompanyInfoID,
		)
		if err != nil {
			return err
		}

	} else {
		//connect to datastore
		client, err := datastoreutils.Connect(c)
		if err != nil {
			return err
		}

		//get full key
		key := datastore.NameKey(datastoreutils.EntityCompanyInfo, datastoreKeyName, nil)

		//save company info
		_, err = client.Put(c, key, &i)
		if err != nil {
			return err
		}
	}

	return nil
}

//SaveDefaultInfo sets some default data when a company first starts using this app
//This func is called when the initial super admin is created.
func SaveDefaultInfo(c context.Context) error {
	err := save(c, defaultCompanyInfo)
	return err
}
