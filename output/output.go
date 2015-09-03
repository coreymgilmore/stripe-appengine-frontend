package output

import (
	"net/http"
	"encoding/json"
	
	"github.com/coreymgilmore/timestamps"
)

//BASIC STRUCT FOR RETURNING DATA TO CLIENT AS JSON
//Ok is used to tell if an error occured
//MsgData is an interface because it could be a struct of any type
type returnObj struct {
	Ok 			bool 		`json:"ok"`
	MsgType		string 		`json:"type"`
	MsgData		interface{}	`json:"data"`
	Datetime 	string 		`json:"datetime"`
}

//OBJECT FOR MSGDATA WHEN AN ERROR OCCURS
//shows info on the error
type errorObj struct {
	Title 	string 		`json:"error_type"`
	Msg 	string	`json:"error_msg"`
}

//**********************************************************************

//RETURN DATA TO CLIENT
//basic boilerplate function
//returns data to client in a consistant json object that is easily checked for errors
//ok is true on successful events, for false when an error occurs
func returnData(ok bool, msgType string, msgData interface{}, resCode int, w http.ResponseWriter) {
	//build data to return as json
	o := returnObj{
		Ok: 		ok,
		MsgType: 	msgType,
		MsgData: 	msgData,
		Datetime: 	timestamps.ISO8601(),
	}

	//set content type
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	//set response code
	w.WriteHeader(resCode)

	//return json to client
	output, _ := json.Marshal(o)
	w.Write(output)
	return
}

//ERRORS
//when an error occurs, data object has error message and info
//sets an http status of error so client does not get a '200'
func Error(title error, msg string, w http.ResponseWriter) {
	//get error as a string
	titleStr := title.Error()

	//error obj
	d := errorObj{
		Title: 	titleStr,
		Msg: 	msg,
	}

	//send message to client
	returnData(false, "error", d, http.StatusBadRequest, w)
	return
}

//SUCCESS
//when a task completed successfully
func Success(msgType string, msgData interface{}, w http.ResponseWriter) {	
	returnData(true, msgType, msgData, http.StatusOK, w)
	return
}
