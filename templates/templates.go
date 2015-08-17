package templates 

import (
	"net/http"
	"html/template"

	"io/ioutil"
	"log"
)

//DIRECTORY STORING TEMPLATES
//path needs to be absolute so program works from anywhere in OS when "go install"-ed
const P = "website/templates/"

//VAR FOR STORING BUILT TEMPLATES
var htmlTemplates *template.Template

//STRUCT FOR HOLDING NOTIFICATION TEMPLATE DATA
type NotificationPage struct {
	PanelColor 		string
	Title 			string
	Message 		interface{}
	BtnColor 		string
	LinkHref 		string
	BtnText 		string
}

//**********************************************************************
//FUNCS

//GET LIST OF FILES FROM DIRECTORY TO BUILD INTO TEMPLATES
//do this instead of having to list every file in ParseFiles() manually
func Build() {
	//placeholder
	filepaths := make([]string, 0, 1)

	//get list of files
	files, err := ioutil.ReadDir(P)
	if err != nil {
		log.Panic(err)
		return
	}

	//get full file paths as strings
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		path := P + f.Name()
		filepaths = append(filepaths, path)
	}

	//parse files into templates
	htmlTemplates = template.Must(template.ParseFiles(
		filepaths...
	))

	return
}

//DISPLAY A TEMPLATE TO THE CLIENT
//data is a struct used to fill in data into the template
func Load(w http.ResponseWriter, templateName string, data interface{}) {
	template := templateName + ".html"

	if err := htmlTemplates.ExecuteTemplate(w, template, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
