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

	out_config,err := loadFromFile(configfilename)

	assert.Nil(t,err)
	assert.Equal(t,out_config,&expectedconfig)
}

func Test_loadFromYAMLFile(t *testing.T) {
	var expectedconfig SloopConfig
	configfilename:="testconfig.yaml"
	configFile, err := ioutil.ReadFile(configfilename)
	err = yaml.Unmarshal(configFile, &expectedconfig)

	out_config,err := loadFromFile(configfilename)

	assert.Nil(t,err)
	assert.Equal(t,out_config,&expectedconfig)
}
