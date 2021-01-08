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
	someContents1   = "contents abcd"
	filePath        = "webfiles/index.html"
	errorString     = "Webfile %v is invalid.  Must start with %v"
)

func Test_BindataReadWebfile_True(t *testing.T) {
	expectedOutput, err := Asset(filePath)
	assert.Nil(t, err)

	actualOutput, _ := readWebfile(filePath, nil, &afero.Afero{afero.NewMemMapFs()})
	assert.Equal(t, expectedOutput, actualOutput)
}

func Test_LocalReadWebfile_True(t *testing.T) {
	notExpectedOutput, _ := Asset(filePath)

	fs := &afero.Afero{afero.NewMemMapFs()}
	writeFile(t, fs, filePath, someContents1)

	actualOutput, _ := readWebfile(filePath, nil, fs)

	assert.NotEqual(t, notExpectedOutput, actualOutput)
	assert.Equal(t, someContents1, actualOutput)
}

func Test_FilenotinReqdFormat_False(t *testing.T) {
	filePath := "index.html"
	_, err := readWebfile(filePath, nil, &afero.Afero{afero.NewMemMapFs()})
	assert.Errorf(t, err, errorString, filePath, prefix)
}

func writeFile(t *testing.T, fs *afero.Afero, filePath string, content string) {
	err := fs.MkdirAll(path.Dir(filePath), defaultFileMode)
	assert.Nil(t, err)
	err = fs.WriteFile(filePath, []byte(content), defaultFileMode)
	assert.Nil(t, err)
}
