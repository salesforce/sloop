package config

import (
	"encoding/json"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func Test_loadFromJSONFile_Success(t *testing.T) {
	var expectedconfig SloopConfig
	configfilename, _ := filepath.Abs("../testFiles/testconfig.json")
	configFile, err := ioutil.ReadFile(configfilename)
	err = json.Unmarshal(configFile, &expectedconfig)
	assert.Nil(t, err)

	out_config, err := loadFromFile(configfilename)

	assert.Nil(t, err)
	assert.Equal(t, out_config, &expectedconfig)
}

func Test_loadFromYAMLFile_Success(t *testing.T) {
	var expectedconfig SloopConfig
	configfilename, _ := filepath.Abs("../testFiles/testconfig.yaml")
	configFile, err := ioutil.ReadFile(configfilename)
	err = yaml.Unmarshal(configFile, &expectedconfig)
	assert.Nil(t, err)

	out_config, err := loadFromFile(configfilename)

	assert.Nil(t, err)
	assert.Equal(t, out_config, &expectedconfig)
}

func Test_loadFromTxtFile_shouldPanic(t *testing.T) {
	configfilename, _ := filepath.Abs("../testFiles/testconfig.txt")
	assert.Panics(t, func() { loadFromFile(configfilename) }, "The code did not panic")
}

func Test_loadFromNoFile_shouldPanic(t *testing.T) {
	configfilename, _ := filepath.Abs("../testconfig.json")
	assert.Panics(t, func() { loadFromFile(configfilename) }, "The code did not panic")
}
