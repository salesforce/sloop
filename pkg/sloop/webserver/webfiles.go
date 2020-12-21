package webserver

import (
	"fmt"
	"html/template"
	"path"
	"strings"
)

const (
	prefix = "webfiles/"
)
// ReadWebfile is a function which finds the webfiles that have been predefined and converted to binary format.
func ReadWebfile(filepath string) ([]byte, error) {
	if !strings.HasPrefix(filepath, prefix) {
		return []byte{}, fmt.Errorf("Webfile %v is invalid.  Must start with %v or %v", filepath, prefix)
	}
	return Asset(filepath)
}

// Example input:
//   templateName = index.html
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