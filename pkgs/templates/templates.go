/*
Package templates handles building and showing HTML files used to build the GUI.
*/
package templates

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
)

//templateDir is the directory where the HTML files are stored
//this is based off of the locaton of the "app.go" and "app.yaml" files.
const templateDir = "./website/templates/"

//htmlTemplates is a variable for holding the built golang templates
var htmlTemplates *template.Template

//NotificationPage is used to display error or warning messages in the GUI
//Defining a struct makes it so the GUI is consistently displayed.
type NotificationPage struct {
	PanelColor string
	Title      string
	Message    interface{}
	BtnColor   string
	LinkHref   string
	BtnText    string
}

//init handles finding the HTML files, parsing them, and building the golang templates.
//This is done when the program first starts.
//Templates are cached for use.
//This func works by checking for files in the templateDir directory, building full paths for each file,
//parsing the files into golang templates, and storing the templates in a variable for future use.
//By checking for files in the templateDir directory this stops us from having to list each file separately
//in template.ParseFiles().
func init() {
	//get list of files in the directory we store the templates in
	files, err := ioutil.ReadDir(templateDir)
	if err != nil {
		log.Panic(err)
		return
	}

	//where we store the full path to each HTML file
	var paths []string

	//get full file paths for each HTML file
	//save the path
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
//This shows an html page to the user == display the GUI.
//Don't need to put ".html" in templateName to reduce retyping elsewhere in this codebase.
func Load(w http.ResponseWriter, templateName string, data interface{}) {
	template := templateName + ".html"

	if err := htmlTemplates.ExecuteTemplate(w, template, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	return
}
