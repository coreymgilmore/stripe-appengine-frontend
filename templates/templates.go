package templates

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
)

//DIRECTORY STORING TEMPLATES
//path needs to be absolute so program works from anywhere in OS when "go install"-ed
const templateDir = "website/templates/"

//VAR FOR STORING BUILT TEMPLATES
var htmlTemplates *template.Template

//STRUCT FOR HOLDING NOTIFICATION TEMPLATE DATA
type NotificationPage struct {
	PanelColor string
	Title      string
	Message    interface{}
	BtnColor   string
	LinkHref   string
	BtnText    string
}

//**********************************************************************
//FUNCS

//GET LIST OF FILES FROM DIRECTORY TO BUILD INTO TEMPLATES
//scans for files in the template directory
//saves each file as a full path to a map (map of strings where each string is a file path)
//then builds the templates with these files
//do this instead of having to list every file in ParseFiles() manually
func Init() {
	//placeholder
	paths := make([]string, 0, 8)

	//get list of files
	files, err := ioutil.ReadDir(templateDir)
	if err != nil {
		log.Panic(err)
		return
	}

	//get full file paths as strings
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		path := templateDir + f.Name()
		paths = append(paths, path)
	}

	//parse files into templates
	htmlTemplates = template.Must(template.ParseFiles(
		paths...,
	))
	return
}

//DISPLAY A TEMPLATE TO THE CLIENT
//data is a struct used to fill in data into the template
//used for actually showing the template to a user
func Load(w http.ResponseWriter, templateName string, data interface{}) {
	template := templateName + ".html"

	if err := htmlTemplates.ExecuteTemplate(w, template, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	return
}
