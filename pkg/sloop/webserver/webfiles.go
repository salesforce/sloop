package webserver

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"path"
	"strings"
	"sort"
)

const (
	prefix = "webfiles/"
)

// go-bindata -o bindata.go webfiles
// ReadWebfile is a function which finds the webfiles that have been predefined and converted to binary format.
// sample input : filepath= "webfiles/index.html"
func ReadWebfile(filepath string) ([]byte, error) {
	if !strings.HasPrefix(filepath, prefix) {
		return []byte{}, fmt.Errorf("Webfile %v is invalid.  Must start with %v",filepath, prefix)
	}

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		files := AssetNames()
		//if file exists in binary form
		if sort.SearchStrings(files, filepath) != 0 {
			return Asset(filepath)
		}else {
			return nil, err
		}
	}
	return data, err

}

// Example input:
// templateName = index.html
// Get Template function creates a new template of the webfile passed as a string after first reading the file by
// calling ReadWebfile ().
func GetTemplate(templateName string) (*template.Template, error) {
	data, err := ReadWebfile(path.Join(prefix, templateName))
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