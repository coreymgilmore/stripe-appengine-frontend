package datastoreutils

import (
	"context"
	"os"

	"cloud.google.com/go/datastore"
)

//projectID is the project we are using this datastore connection for
//this is provided in app.yaml environmental variable
//we store this as a package level variable for reuse if needed
var projectID string

//Client is the connection to the datastore
//we save the connection once it is established so we don't need to re-establish it
//every time
var Client *datastore.Client

//Connect connects to the datastore
func Connect() error {
	//get project id from app.yaml
	projectID = os.Getenv("PROJECT_ID")

	//connect to datastore
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return nil
	}

	//save the client
	Client = client

	return nil
}
