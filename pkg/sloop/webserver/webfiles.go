package webserver

import (
	"fmt"
	_ "github.com/jteeuwen/go-bindata"
	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/spf13/afero"
	"html/template"
)

const (
	prefix      = "webfiles/"
	errorString = "Webfile %v does not exist in local directory or in binary form."
)

// go-bindata -o bindata.go webfiles
// ReadWebfile is a function which finds the webfiles that have been predefined and converted to binary format.
// sample input : fileName="index.html"
func readWebfile(fileName string, fs *afero.Afero) ([]byte, error) {
	//file exists as a physical file
	data, err := fs.ReadFile(common.GetFilePath(webFilesPath, fileName))
	if err == nil {
		return data, err
	} else {
		//file exists in binary
		binFileList := AssetNames()
		if common.Contains(binFileList, common.GetFilePath(prefix, fileName)) {
			return Asset(common.GetFilePath(prefix, fileName))
		}
	}
	return nil, fmt.Errorf(errorString, fileName)
}

// Example input:
// templateName = index.html
// Get Template function creates a new template of the webfile passed as a string after first reading the file by
// calling ReadWebfile ().
func getTemplate(templateName string, _ []byte) (*template.Template, error) {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	data, err := readWebfile(templateName, &fs)
	if err == nil {
		newTemplate := template.New(templateName)
		newTemplate, err = newTemplate.Parse(string(data))
		return newTemplate, nil
	}
	return nil, err
}
