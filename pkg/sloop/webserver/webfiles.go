package webserver

import (
	"fmt"
	"html/template"

	_ "github.com/jteeuwen/go-bindata"
	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/spf13/afero"
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
	if err != nil {
		//file exists in binary
		binFileList := AssetNames()
		binFileName := common.GetFilePath(prefix, fileName)
		if common.Contains(binFileList, binFileName) {
			return Asset(binFileName)
		}
		return nil, fmt.Errorf(errorString, fileName)
	}
	return data, err
}

// Example input:
// templateName = index.html
// Get Template function creates a new template of the webfile passed as a string after first reading the file by
// calling ReadWebfile ().
func getTemplate(templateName string, _ []byte) (*template.Template, error) {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	data, err := readWebfile(templateName, &fs)
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
