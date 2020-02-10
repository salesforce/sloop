package storemanager

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_boolToFloat(t *testing.T) {
	assert.Equal(t, float64(1), boolToFloat(true))
	assert.Equal(t, float64(0), boolToFloat(false))
}
