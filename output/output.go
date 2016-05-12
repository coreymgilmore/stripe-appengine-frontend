/*
Package output is used to send data back to client in a consistent manner.
The data send back is always sent back as JSON.
Sending data back to client using these funcs allows the data to be easily parsable by
the client since the format is always the same.

Responses can either be successful or error. Each has specific uses and returns
data slightly differently.
*/

package output

import (
	"encoding/json"
	"github.com/coreymgilmore/timestamps"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"net/http"
)

//returnObj is the basic format for returning data to the client
//this is converted to json before it is sent
type returnObj struct {
	//true if this is a "succcess" message being returnd
	//false if this is an "error"
	Ok bool `json:"ok"`

	MsgType  string      `json:"type"`
	MsgData  interface{} `json:"data"`
	Datetime string      `json:"datetime"`
}

//errorObj is the MsgData when an error is being returned
//this hold some descriptive data on the error that occured
type errorObj struct {
	Title string `json:"error_type"`
	Msg   string `json:"error_msg"`
}

//returnData is a low level func that actually sends the data back to the client
//this sends the response
//ok is true if the message is "success", false if it is "error"
//resCode is an http response code
func returnData(ok bool, msgType string, msgData interface{}, resCode int, w http.ResponseWriter) {
	//build data to return as json
	o := returnObj{
		Ok:       ok,
		MsgType:  msgType,
		MsgData:  msgData,
		Datetime: timestamps.ISO8601(),
	}

	//set content type
	//since we only want the data to be interpreted as json since that is the only type of data we send back
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	//set response code
	w.WriteHeader(resCode)

	//return json to client
	output, _ := json.Marshal(o)
	w.Write(output)
	return
}

//Error is used when an error occured within the app and we could not continue with the task
//this returns some data on the error so the user can diagnose it or contact an admin
//this logs to appengine logs (viewable in google cloud platform) so admins can also see the error and more details
//responds 400 status code since this clearly was not an "ok" event
func Error(title error, msg string, w http.ResponseWriter, r *http.Request) {
	//get error as a string
	titleStr := title.Error()

	//error obj
	d := errorObj{
		Title: titleStr,
		Msg:   msg,
	}

	//log errors into appengine log
	//"%+v" is formatting for the error
	c := appengine.NewContext(r)
	log.Errorf(c, "%+v", "output.Error:", d)

	//send message to client
	returnData(false, "error", d, http.StatusBadRequest, w)
	return
}

//Success is used when no errors occured and we want to send data back to the client
//the msgData could be blank/empty if the user was making a request in which all the client looks for is a status ok
//sometimes no data is returned on purpose
func Success(msgType string, msgData interface{}, w http.ResponseWriter) {
	returnData(true, msgType, msgData, http.StatusOK, w)
	return
}
