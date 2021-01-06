package webserver

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)
const (
	defaultFileMode = os.FileMode(0755)
	someContents1 = "contents abcd"
	filepath = "webfiles/index.html"
)

func Test_BindataReadWebfile_True(t *testing.T) {
	expectedOutput, err := Asset(filepath)
	assert.Nil(t,err)

	actualOutput, _ := readWebfile(filepath,nil,&afero.Afero{afero.NewMemMapFs()})
	assert.Equal(t,actualOutput,expectedOutput)
}

func Test_LocalReadWebfile_True(t *testing.T){
	notExpectedOutput, _ := Asset(filepath)

	fs := &afero.Afero{afero.NewMemMapFs()}
	writeFile(t,fs, filepath,someContents1)

	actualOutput,_ :=readWebfile("webfiles/index.html",nil,fs)

	assert.NotEqual(t,notExpectedOutput,actualOutput)

}

func Test_FilenotinReqdFormat_False(t *testing.T) {
	filepath :="index.html"
	_, err := readWebfile(filepath,nil,&afero.Afero{afero.NewMemMapFs()})
	assert.Errorf(t,err,"Webfile %v is invalid.  Must start with %v",filepath,prefix)
}

func writeFile(t *testing.T, fs *afero.Afero, filePath string, content string) {
	err := fs.MkdirAll(path.Dir(filePath), defaultFileMode)
	assert.Nil(t, err)
	err = fs.WriteFile(filePath, []byte(content), defaultFileMode)
	assert.Nil(t, err)
}




