package webserver

import (
	"fmt"
	_ "github.com/jteeuwen/go-bindata"
	"github.com/spf13/afero"
	"html/template"
	"path"
	"strings"
)

const (
	prefix      = "webfiles/"
	errorString = "Webfile %v is invalid.  Must start with %v"
)

// go-bindata -o bindata.go webfiles
// ReadWebfile is a function which finds the webfiles that have been predefined and converted to binary format.
// sample input : filepath= "webfiles/index.html"
func readWebfile(filepath string, fs *afero.Afero) ([]byte, error) {
	if !strings.HasPrefix(filepath, prefix) {
		return nil, fmt.Errorf(errorString, filepath, prefix)
	}
	data, err := fs.ReadFile(filepath)
	if err == nil {
		return data, err
	}
	files := AssetNames()
	//if file exists in binary form
	if contains(files,filepath)  {
		return Asset(filepath)
	} else {
		return nil, err
	}
}

// Example input:
// templateName = index.html
// Get Template function creates a new template of the webfile passed as a string after first reading the file by
// calling ReadWebfile ().
func getTemplate(templateName string, _ []byte) (*template.Template, error) {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	data, err := readWebfile((path.Join(prefix, templateName)), &fs)
	if err != nil {
		return nil, err
	}
	newTemplate := template.New(templateName)
	newTemplate, err = newTemplate.Parse(string(data))
	if err != nil {
		return nil, err
	}
	return newTemplate, nil
}

func contains(list []string, elem string) bool {
	for _,str := range list {
		if str == elem {
			return true
		}
	}
	return false
}