package webserver

import (
	"fmt"
	_ "github.com/jteeuwen/go-bindata"
	"github.com/spf13/afero"
	"html/template"
	"path"
	"sort"
	"strings"
)

const (
	prefix = "webfiles/"
)

// go-bindata -o bindata.go webfiles
// ReadWebfile is a function which finds the webfiles that have been predefined and converted to binary format.
// sample input : filepath= "webfiles/index.html"
func readWebfile(filepath string, _, fs *afero.Afero) ([]byte, error) {
	if !strings.HasPrefix(filepath, prefix) {
		return []byte{}, fmt.Errorf("Webfile %v is invalid.  Must start with %v", filepath, prefix)
	}
	data, err := fs.ReadFile(filepath)
	if err != nil {
		files := AssetNames()
		//if file exists in binary form
		if sort.SearchStrings(files, filepath) != 0 {
			return Asset(filepath)
		} else {
			return nil, err
		}
	}
	return data, err

}

// Example input:
// templateName = index.html
// Get Template function creates a new template of the webfile passed as a string after first reading the file by
// calling ReadWebfile ().
func getTemplate(templateName string, _ []byte) (*template.Template, error) {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	data, err := readWebfile((path.Join(prefix, templateName)), nil, &fs)
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
