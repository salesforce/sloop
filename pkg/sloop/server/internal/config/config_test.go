package config

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
	"encoding/json"
	"github.com/ghodss/yaml"
)

func Test_loadFromJSONFile(t *testing.T) {
	var expectedconfig SloopConfig
	configfilename:="testconfig.json"
	configFile, err := ioutil.ReadFile(configfilename)
	err = json.Unmarshal(configFile, &expectedconfig)
	assert.Nil(t,err)

	out_config,err := loadFromFile(configfilename)

	assert.Nil(t,err)
	assert.Equal(t,out_config,&expectedconfig)
}

func Test_loadFromYAMLFile(t *testing.T) {
	var expectedconfig SloopConfig
	configfilename:="testconfig.yaml"
	configFile, err := ioutil.ReadFile(configfilename)
	err = yaml.Unmarshal(configFile, &expectedconfig)
	assert.Nil(t,err)

	out_config,err := loadFromFile(configfilename)

	assert.Nil(t,err)
	assert.Equal(t,out_config,&expectedconfig)
}

func Test_loadFromTxtFile(t *testing.T) {
	configfilename:="testconfig.txt"
	_,err := loadFromFile(configfilename)

	assert.Nil(t,err)

}
func Test_loadFromNoFile(t *testing.T) {
	configfilename:="config.json"
	_,err := loadFromFile(configfilename)

	assert.Nil(t,err)

}
