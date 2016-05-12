/*
Package templates reads through the available html templates, parses them, build the golang templates,
caches the templates for future use, and returns the build templates to the user when needed.

Templates are stored in another directory for better organizing of code. Templates are just html files
with golang templating code built into them.
*/

package templates

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
)

//directory where template files are saved
const templateDir = "website/templates/"

//global variable for storing built templates
var htmlTemplates *template.Template

//struct for holding notification template data
type NotificationPage struct {
	PanelColor string
	Title      string
	Message    interface{}
	BtnColor   string
	LinkHref   string
	BtnText    string
}

//Init reads the templates files from a directory, parses them, and builds templates to be used in the future
//  scans for files in the template directory
//  saves each file as a full path to a map (map of strings where each string is a file path)
//  then builds the templates with these files
//  do this instead of having to list every file in ParseFiles() manually
func Init() {
	//get list of files
	files, err := ioutil.ReadDir(templateDir)
	if err != nil {
		log.Panic(err)
		return
	}

	//placeholder
	paths := make([]string, 0, 8)

	//get full file paths as strings
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		path := templateDir + f.Name()
		paths = append(paths, path)
	}

	//parse files into templates
	htmlTemplates = template.Must(template.ParseFiles(paths...))
	return
}

//Load shows a template to the client
//this "shows" and html page
func Load(w http.ResponseWriter, templateName string, data interface{}) {
	template := templateName + ".html"

	if err := htmlTemplates.ExecuteTemplate(w, template, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	return
}
