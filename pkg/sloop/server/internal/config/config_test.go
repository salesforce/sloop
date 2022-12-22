package config

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
)

func Test_loadFromJSONFile_Success(t *testing.T) {
	var expectedconfig SloopConfig
	configfilename, _ := filepath.Abs("../testFiles/testconfig.json")
	configFile, err := ioutil.ReadFile(configfilename)
	err = json.Unmarshal(configFile, &expectedconfig)
	assert.Nil(t, err)

	var outConfig *SloopConfig
	outConfig = loadFromFile(configfilename, outConfig)
	assert.Equal(t, &expectedconfig, outConfig)
}

func Test_loadFromYAMLFile_Success(t *testing.T) {
	var expectedconfig SloopConfig
	configfilename, _ := filepath.Abs("../testFiles/testconfig.yaml")
	configFile, err := ioutil.ReadFile(configfilename)
	err = yaml.Unmarshal(configFile, &expectedconfig)
	assert.Nil(t, err)

	var outConfig *SloopConfig
	outConfig = loadFromFile(configfilename, outConfig)
	assert.Equal(t, &expectedconfig, outConfig)
}

func Test_loadFromTxtFile_shouldPanic(t *testing.T) {
	var config *SloopConfig
	configfilename, _ := filepath.Abs("../testFiles/testconfig.txt")
	assert.Panics(t, func() { loadFromFile(configfilename, config) }, "The code did not panic")
}

func Test_loadFromNoFile_shouldPanic(t *testing.T) {
	var config *SloopConfig
	configfilename, _ := filepath.Abs("../testconfig.json")
	assert.Panics(t, func() { loadFromFile(configfilename, config) }, "The code did not panic")
}
