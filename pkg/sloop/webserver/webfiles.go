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

func ReadWebfile(filepath string) ([]byte, error) {
	if !strings.HasPrefix(filepath, prefix) {
		return []byte{}, fmt.Errorf("Webfile %v is invalid.  Must start with %v or %v", filepath, prefix)
	}
	return Asset(filepath)
}

// Example input:
//   templateName = index.html
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