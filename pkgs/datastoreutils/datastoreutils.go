package datastoreutils

import (
	"context"
	"errors"

	"cloud.google.com/go/datastore"
)

//config is the set of configuration options for the datastore
//this struct is used when SetConfig is run in package main init()
type config struct {
	ProjectID   string //the project on Google Cloud and noted in app.yaml
	Development bool   //set to true in develop to not use live data
}

//Config is a copy of the config struct with some defaults set
var Config = config{
	ProjectID:   "",
	Development: false,
}

//configuration errors
var (
	errInvalidProjectID = errors.New("datastoreutils: A project ID wasn't given or is invalid")
)

//these are the names of the entity types in the Google Cloud Datastore
//entity types are like tables
//variables, not constants, because we can edit them in SetConfig
var (
	EntityUsers       = "users"
	EntityCards       = "card"
	EntityCompanyInfo = "companyInfo"
	EntityAppSettings = "appSettings"
)

//SetConfig saves the configuration for the datastore
func SetConfig(c config) error {
	//validate config options
	if len(c.ProjectID) < 1 {
		return errInvalidProjectID
	}

	//rename entity types if we are in dev mode
	if c.Development {
		EntityUsers = "dev-" + EntityUsers
		EntityCards = "dev-" + EntityCards
		EntityCompanyInfo = "dev-" + EntityCompanyInfo
		EntityAppSettings = "dev-" + EntityAppSettings
	}

	//save config to package variable
	Config = c

	return nil
}

//Connect connects to the datastore
func Connect(c context.Context) (client *datastore.Client, err error) {
	client, err = datastore.NewClient(c, Config.ProjectID)
	return
}

//GetKeyFromID gets the full datastore key from the datastore id for a given entity type
//id is a numeric value generated automatically by the datastore
func GetKeyFromID(entityType string, id int64) *datastore.Key {
	return datastore.IDKey(entityType, id, nil)
}

//GetKeyFromName gets the full datastore key from the key's name for a given entity type
//name is an alphanumeric value we provide
func GetKeyFromName(entityType, keyName string) *datastore.Key {
	return datastore.NameKey(entityType, keyName, nil)
}

//GetNewIncompleteKey generates a new incomplete key for an entity being saved to the datastore
//for a given entity type
func GetNewIncompleteKey(entityType string) *datastore.Key {
	return datastore.IncompleteKey(entityType, nil)
}
