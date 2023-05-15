package common

import (
	"fmt"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

var files = []string{"webfiles/index.html", "webfiles/debug.html", "webfiles/filter.js", "webfiles/sloop.cs"}
var filePath = "webfiles/index.html"
var filePath2 = "webfiles/sloop.css"
var fileName = "index.html"
var filePrefix = "webfiles/"

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

func Test_GetFilePath(t *testing.T) {
	expectedOutput := path.Join(filePrefix, fileName)
	actualOutput := GetFilePath(filePrefix, fileName)
	assert.Equal(t, expectedOutput, actualOutput)
}

func Test_Truncate_StringLongerThanWidth(t *testing.T) {
	stringLong := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Donec eget odio quis felis laoreet dictum."
	expectedOutput := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Donec eget odio quis..."
	actualOutput, _ := Truncate(stringLong, 80)
	assert.Equal(t, expectedOutput, actualOutput)
}

func Test_Truncate_StringShorterThanWidth(t *testing.T) {
	stringMedium := "Lorem ipsum dolor"
	expectedOutput := "Lorem ipsum dolor"
	actualOutput, _ := Truncate(stringMedium, 80)
	assert.Equal(t, expectedOutput, actualOutput)
}

func Test_Truncate_WidthShorterThanDelimiter(t *testing.T) {
	stringShort := "Lorem"
	expectedOutput := "..."
	actualOutput, _ := Truncate(stringShort, 1)
	assert.Equal(t, expectedOutput, actualOutput)
}

func Test_Truncate_StringEmpty(t *testing.T) {
	stringEmpty := ""
	expectedOutput := ""
	actualOutput, _ := Truncate(stringEmpty, 1)
	assert.Equal(t, expectedOutput, actualOutput)
}
