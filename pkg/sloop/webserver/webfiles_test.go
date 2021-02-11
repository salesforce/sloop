package webserver

import (
	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

const (
	defaultFileMode = os.FileMode(0755)
	someContents1   = "contents abcd"
	filePath        = "webfiles/index.html"
	fileName        = "index.html"
)

func Test_BindataReadWebfile_True(t *testing.T) {
	expectedOutput, err := Asset(filePath)
	assert.Nil(t, err)

	actualOutput, _ := readWebfile(fileName, &afero.Afero{afero.NewMemMapFs()})
	assert.Equal(t, expectedOutput, actualOutput)
}

func Test_LocalReadWebfile_True(t *testing.T) {
	notExpectedOutput, _ := Asset(filePath)

	fs := &afero.Afero{afero.NewMemMapFs()}
	fullPath := common.GetFilePath(webFilesPath, fileName)
	writeFile(t, fs, fullPath, someContents1)

	actualOutput, _ := readWebfile(fileName, fs)

	assert.NotEqual(t, notExpectedOutput, actualOutput)
	assert.Equal(t, []uint8(someContents1), actualOutput)
}

func Test_FileNotinLocalOrBin(t *testing.T) {
	fileName := "blah.html"
	_, err := readWebfile(fileName, &afero.Afero{afero.NewMemMapFs()})
	assert.Errorf(t, err, errorString, fileName)
}

func writeFile(t *testing.T, fs *afero.Afero, filePath string, content string) {
	err := fs.MkdirAll(path.Dir(filePath), defaultFileMode)
	assert.Nil(t, err)
	err = fs.WriteFile(filePath, []byte(content), defaultFileMode)
	assert.Nil(t, err)
}
