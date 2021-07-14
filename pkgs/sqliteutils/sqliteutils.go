/*
Package sqliteutils is used to interact with a sqlite db.
*/
package sqliteutils

import (
	"log"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" //db driver
)

//Connection is a global variable for using a connection to the db.
//This is what we use to run queries on the database.
//This is a "pooled" connection.
var Connection *sqlx.DB

//config is the set of configuration options for interacting with a sqlite db.
//this struct is used in SetConfig which is run in package main init()
type config struct {
	PathToDatabaseFile string //the full path to the file used as the database, defaults to $GOPATH/src/github.com/coreymgilmore/stripe-appengine-frontend/services/process-cards/sqlite.db
	UseSQLite          bool   //set to true when we want to use sqlite as the storage engine, so we know to use sqlite versus cloud datastore
}

//Config is a copy of the config with some defaults set
var Config = config{
	PathToDatabaseFile: "",
	UseSQLite:          false,
}

//dbType is the type of database we are connecting to
const dbType = "sqlite3"

//defaultDatabasePartialPath is the default location of the file used to store the sqlite db
//this is a partial path because we have to prepend the $GOPATH to it
const defaultDatabasePartialPath = "/src/github.com/coreymgilmore/stripe-appengine-frontend/services/process-cards/sqlite.db"

//SetConfig saved the configuration options for using sqlite
func SetConfig(c config) {
	//check if user provided a path to the db file or if we should use default path
	if c.PathToDatabaseFile == "" {
		c.PathToDatabaseFile = os.Getenv("GOPATH") + filepath.FromSlash(defaultDatabasePartialPath)
	} else {
		c.PathToDatabaseFile = filepath.FromSlash(c.PathToDatabaseFile)
	}

	//save the configuration
	c.UseSQLite = true
	Config = c

	log.Println("Using SQLite file:", Config.PathToDatabaseFile)
}

func init() {
	RegisterDeployFunc(
		CreateTableUsers,
		CreateTableCard,
		CreateTableCompanyInfo,
		CreateTableAppSettings,
	)
}

//Bindvars is used to hold the SQL query parameters
//parameters are used in WHERE, INSERT, UPDATE, etc.
type Bindvars []interface{}

//Connect establishes and tests a connection to a db
//if this returns successfully, queries can be run on the db.
func Connect() {
	//check if the db doesn't exist
	//sqlite db is simply a file
	if _, err := os.Stat(Config.PathToDatabaseFile); os.IsNotExist(err) {
		deployDB()
	}

	//connect to db
	c, err := sqlx.Open(dbType, Config.PathToDatabaseFile)
	if err != nil {
		log.Fatalln("sqliteutils.Connect: Could not open connection to sqlite db.", err)
		log.Println("Attempted to use path:", Config.PathToDatabaseFile)
		return
	}

	//test connection
	//this actually uses the connection to make sure it works
	if err := c.Ping(); err != nil {
		log.Fatalln("sqliteutils.Connect: Could not test connection to db.", err)
		return
	}

	//set the mapper for mapping column names to struct fieldnames
	//unless we override this with struct tags, a column name MUST be the same as the stuct field name
	//doing this reduces the amount of struct tags we need to add and makes matching struct fields to columns really easy
	//this can be overridden after the db connection is established (ex.: in init() in main.go) by redefining the MapperFunc
	c.MapperFunc(func(s string) string { return s })

	//save connection to global var so we can reuse it
	Connection = c

	//make sure schema is correct
	err = AddColumnLastUsedTimestamp(c)
	if err != nil {
		log.Fatalln(err)
		return
	}

	log.Println("sqliteutils.Connect: Connecting...done")
}

//deployDB deploys the db
func deployDB() {
	log.Println("sqliteutils: Deploying db...")

	c, err := sqlx.Open(dbType, Config.PathToDatabaseFile)
	if err != nil {
		log.Fatalln("sqliteutils.deployDB: Could not open connection to sqlite db to deploy.", err)
		log.Println("Attempted to use path:", Config.PathToDatabaseFile)
		return
	}
	defer c.Close()

	//iterate through deploy funcs
	for _, f := range deployFuncs {
		if err := f(c); err != nil {
			log.Fatalln(err)
		}
	}

	log.Println("sqliteutils.deployDB: Deploying db...done")
}

//Close closes the connection to the database.
func Close() {
	Connection.Close()
}

//deployFunc is the signature for a function that is used to deploy the schema of the db.
//This could be used to create a table, insert initial data, etc.
//The "create table" or "insert initial data" func on the file that defines a tables
//schema should be of this format.  These funcs are registered in main.go init() [using
//RegisterDeployFunc() below] and called during deployDatabase().
type deployFunc func(c *sqlx.DB) error

//deployFuncs is a list of functions that create tables and insert initial data into tables.
//This list of funcs will be run when a database is deployed and cleans up code since we dont have
//to have an "if err" block for every table that needs to be created.
var deployFuncs []deployFunc

//RegisterDeployFunc saves a func that is used to deploy the database to the deployFuncs
//variable so we can use this func when deploying the database.  Using this func makes
//it so the deployFunc type or the deployFuncs list of funcs isn't accessible outside
//of this package.
func RegisterDeployFunc(f ...deployFunc) {
	deployFuncs = append(deployFuncs, f...)
}
