package datastoreutils

import (
	"context"
	"errors"

	"cloud.google.com/go/datastore"
)

//config is the set of configuration options for the datastore
//this struct is used when SetConfig is run in package main init()
type config struct {
	ProjectID string //the project on Google Cloud and noted in app.yaml
}

//Config is a copy of the config struct with some defaults set
var Config = config{
	ProjectID: "",
}

//configuration errors
var (
	errInvalidProjectID = errors.New("datastoreutils: A project ID wasn't given or is invalid")
)

//SetConfig saves the configuration for the datastore
func SetConfig(c config) error {
	//validate config options
	if len(c.ProjectID) < 1 {
		return errInvalidProjectID
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
