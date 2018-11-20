/*
Package sqliteutils is used to interact with a sqlite db.

This file holds the SQL code to deploy a new copy of the database.  The SQL code
should roughly match the fields in the structs used to interact with the database
data, ex: card.CustomerDatastore.
*/
package sqliteutils

import (
	"errors"
	"log"
	"strconv"

	"github.com/coreymgilmore/stripe-appengine-frontend/pkgs/timestamps"
	"github.com/jmoiron/sqlx"
)

//these are the names of the tables used to store data
//these values should match the entity names in datastoreutils.go
const (
	TableUsers       = "users"
	TableCards       = "card"
	TableCompanyInfo = "companyInfo"
	TableAppSettings = "appSettings"
)

//these are the default IDs of the rows in the companyInfo and appSettings tables
//these tables will only ever have one record, so we know what the ID should be
const (
	DefaultCompanyInfoID = 1
	DefaultAppSettingsID = 1
)

//deployment or alter schema errors
var (
	errNoSuchColumn = errors.New("no such column") //this is the exact text that is returned from a SELECT query where a column doesn't exist in the table
)

//CreateTableUsers creates the users table
func CreateTableUsers(c *sqlx.DB) error {
	q := `
		CREATE TABLE IF NOT EXISTS ` + TableUsers + `(
			ID INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			Username TEXT NOT NULL,
			Password TEXT NOT NULL,
			AddCards BOOL NOT NULL,
			RemoveCards BOOL NOT NULL,
			ChargeCards BOOL NOT NULL,
			ViewReports BOOL NOT NULL,
			Administrator BOOL NOT NULL,
			Active BOOL NOT NULL,
			Created TEXT NOT NULL
		)
	`

	_, err := c.Exec(q)
	log.Println("sqliteutils.CreateTableUsers...done")
	return err
}

//CreateTableCard creates the card table
func CreateTableCard(c *sqlx.DB) error {
	q := `
		CREATE TABLE IF NOT EXISTS ` + TableCards + `(
			ID INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			CustomerID TEXT NOT NULL,
			CustomerName TEXT NOT NULL,
			Cardholder TEXT NOT NULL,
			CardExpiration TEXT NOT NULL,
			CardLast4 TEXT NOT NULL,
			StripeCustomerToken TEXT NOT NULL,
			DatetimeCreated TEXT NOT NULL,
			AddedByUser TEXT NOT NULL,
			LastUsedTimestamp INTEGER NOT NULL
		)
	`

	_, err := c.Exec(q)
	log.Println("sqliteutils.CreateTableCard...done")
	return err
}

//CreateTableCompanyInfo creates the companyInfo table
//there should only ever be one record in this table
func CreateTableCompanyInfo(c *sqlx.DB) error {
	q := `
		CREATE TABLE IF NOT EXISTS ` + TableCompanyInfo + `(
			ID INTEGER NOT NULL DEFAULT 1,
			CompanyName TEXT NOT NULL,
			Street TEXT NOT NULL,
			Suite TEXT NOT NULL,
			City TEXT NOT NULL,
			State TEXT NOT NULL,
			PostalCode TEXT NOT NULL,
			Country TEXT NOT NULL,
			PhoneNum TEXT NOT NULL,
			Email TEXT NOT NULL,
			PercentFee REAL NOT NULL,
			FixedFee REAL NOT NULL,
			StatementDescriptor TEXT NOT NULL
		)
	`

	_, err := c.Exec(q)
	if err != nil {
		log.Println("sqliteutils.CreateTableCompanyInfo: creating table", err)
		return err
	}

	//save the first and only record
	q = `
		INSERT INTO ` + TableCompanyInfo + ` (
			ID,
			CompanyName,
			Street,
			Suite,
			City,
			State,
			PostalCode,
			Country,
			PhoneNum,
			Email,
			PercentFee,
			FixedFee,
			StatementDescriptor
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	stmt, err := c.Prepare(q)
	if err != nil {
		log.Println("sqliteutils.CreateTableCompanyInfo: inserting initial data 1", err)
		return err
	}

	_, err = stmt.Exec(
		DefaultCompanyInfoID,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
	)
	if err != nil {
		log.Println("sqliteutils.CreateTableCompanyInfo: inserting initial data 2", err)
		return err
	}

	log.Println("sqliteutils.CreateTableCompanyInfo...done")
	return err
}

//CreateTableAppSettings creates the card table
//there should only ever be one record in this table
func CreateTableAppSettings(c *sqlx.DB) error {
	q := `
		CREATE TABLE IF NOT EXISTS ` + TableAppSettings + `(
			ID INTEGER NOT NULL DEFAULT 1,
			RequireCustomerID BOOL NOT NULL,
			CustomerIDFormat TEXT NOT NULL,
			CustomerIDRegex TEXT NOT NULL,
			ReportTimezone TEXT NOT NULL,
			APIKey TEXT NOT NULL
		)
	`

	_, err := c.Exec(q)
	if err != nil {
		log.Println("sqliteutils.CreateTableAppSettings: creating table", err)
		return err
	}

	//save the first and only record
	q = `
	INSERT INTO ` + TableAppSettings + ` (
		ID,
		RequireCustomerID,
		CustomerIDFormat,
		CustomerIDRegex,
		ReportTimezone,
		APIKey
	) VALUES (?, ?, ?, ?, ?, ?)
	`
	stmt, err := c.Prepare(q)
	if err != nil {
		log.Println("sqliteutils.CreateTableAppSettings: inserting initial data 1", err)
		return err
	}

	_, err = stmt.Exec(
		DefaultAppSettingsID,
		"",
		"",
		"",
		"",
		"",
	)
	if err != nil {
		log.Println("sqliteutils.CreateTableAppSettings: inserting initial data 2", err)
		return err
	}

	log.Println("sqliteutils.CreateTableAppSettings...done")
	return err
}

//AddColumnLastUsedTimestamp adds the LastUsedTimestamp column card table if it doesn't already exist
func AddColumnLastUsedTimestamp(c *sqlx.DB) error {
	//column to add
	column := "LastUsedTimestamp"

	//check if column already exists
	q := `
		SELECT ` + column + ` 
		FROM ` + TableCards + `
		WHERE ID > 0
		LIMIT 1
	`
	var rowValue int64
	err := c.Get(&rowValue, q)
	if err == nil {
		//column exists, do nothing
		return nil
	} else if err.Error() == errNoSuchColumn.Error()+": "+column {
		//column doesn't exist add it
		log.Println("sqliteutils.AddColumnLastUsedTimestamp - column doesn't exist, will be added")
	} else if err != nil {
		//another error occured
		log.Println("sqliteutils.AddColumnLastUsedTimestamp", err)
		return err
	}

	//add the new column
	//set the current unix timestamp as a default value so we can check existing cards for age in
	q = `
		ALTER TABLE ` + TableCards + ` 
		ADD COLUMN ` + column + ` INTEGER DEFAULT ` + strconv.FormatInt(timestamps.Unix(), 10)
	_, err = c.Exec(q)
	log.Println("sqliteutils.AddColumnLastUsedTimestamp...done")
	return err
}
