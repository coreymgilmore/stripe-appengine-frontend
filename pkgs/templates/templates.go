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

//htmlTemplates is a variable for holding the built golang templates
var htmlTemplates *template.Template

//NotificationPage is used to display error or warning messages in the GUI
//Defining a struct makes it so the GUI is consistently displayed.
type NotificationPage struct {
	PanelColor string
	Title      string
	Message    interface{} //interface{} because this could be a string or an error
	BtnColor   string
	LinkHref   string
	BtnText    string
}

//config is the set of configuration options for serving html templates
//this struct is used in SetConfig is run in package main init()
type config struct {
	PathToTemplates string //path to the templates/ directory
	Development     bool   //set to true shows a "in dev mode" header on pages
	UseLocalFiles   bool   //set to true to use local copies of jquery, bootstrap, etc.
}

//Config is a copy of the config struct with some defaults set
var Config = config{
	PathToTemplates: "./website/templates/",
	Development:     false,
	UseLocalFiles:   false,
}

//SetConfig saves the configuration options for serving templates/html pages
func SetConfig(c config) {
	Config = c

	build()
	return
}

//build handles finding the HTML files, parsing them, and building the golang templates.
//This is done when the program first starts.
//Templates are cached for use.
//This func works by checking for files in the templateDir directory, building full paths for each file,
//parsing the files into golang templates, and storing the templates in a variable for future use.
//By checking for files in the templateDir directory this stops us from having to list each file separately
//in template.ParseFiles().
func build() {
	//get list of files in the directory we store the templates in
	files, err := ioutil.ReadDir(Config.PathToTemplates)
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

		path := Config.PathToTemplates + f.Name()
		paths = append(paths, path)
	}

	//parse files into templates
	htmlTemplates = template.Must(template.ParseFiles(paths...))
	return
}

//Load shows a template to the client, show the GUI
func Load(w http.ResponseWriter, templateName string, data interface{}) {
	//build data struct for serving template
	//this takes the data value and any configuration options and combines them
	d := struct {
		Data          interface{}
		Configuration config
	}{
		Data:          data,
		Configuration: Config,
	}

	template := templateName + ".html"
	if err := htmlTemplates.ExecuteTemplate(w, template, d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	return
}
