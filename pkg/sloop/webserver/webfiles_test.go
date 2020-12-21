package webserver

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_InBindata_True(t *testing.T) {
	filepath :="webfiles/index.html"
	expectedOutput, err := Asset(filepath)
	assert.Nil(t,err)

	actualOutput, _ := ReadWebfile(filepath)
	assert.Equal(t,actualOutput,expectedOutput)
}

func Test_FilenotinReqdFormat_False(t *testing.T) {
	filepath :="pkg/sloop/webfiles/index.html"
	_, err := ReadWebfile(filepath)
	assert.Errorf(t,err,"Webfile %v is invalid.  Must start with %v",filepath,prefix)
}




