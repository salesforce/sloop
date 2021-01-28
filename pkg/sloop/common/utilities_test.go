package common

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

var files = []string{"webfiles/index.html", "webfiles/debug.html", "webfiles/filter.js", "webfiles/sloop.cs"}
var filePath = "webfiles/index.html"
var filePath2 = "webfiles/sloop.css"

func Test_boolToFloat(t *testing.T) {
	assert.Equal(t, float64(1), BoolToFloat(true))
	assert.Equal(t, float64(0), BoolToFloat(false))
}

func Test_ParseKey_2_Parts(t *testing.T) {
	keyWith2Parts := "/part1/part2"
	err, _ := ParseKey(keyWith2Parts)

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Errorf("key should have 6 parts: %v", keyWith2Parts), err)
}

func Test_ParseKey_Start_Parts(t *testing.T) {
	keyWith2Parts := "part1/part2/part3/part4/part5/part6/part7"
	err, _ := ParseKey(keyWith2Parts)

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Errorf("key should start with /: %v", keyWith2Parts), err)
}

func Test_ParseKey_Success(t *testing.T) {
	keyWith2Parts := "/part1/part2/part3/part4/part5/part6"
	err, parts := ParseKey(keyWith2Parts)

	assert.Nil(t, err)
	assert.Equal(t, 7, len(parts))
}

func Test_FileExistsInList_True(t *testing.T) {
	expectedOutput := true
	actualOutput := Contains(files, filePath)
	assert.Equal(t, expectedOutput, actualOutput)
}

func Test_FileExistsInList_False(t *testing.T) {
	expectedOutput := false
	actualOutput := Contains(files, filePath2)
	assert.Equal(t, expectedOutput, actualOutput)
}
