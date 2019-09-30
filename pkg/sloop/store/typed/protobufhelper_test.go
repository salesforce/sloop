/**
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_StringToProtobufTimestamp_Success(t *testing.T) {
	expectedResult := &timestamp.Timestamp{
		Seconds: 1562962332,
		Nanos:   0,
	}
	ts, err := StringToProtobufTimestamp("2019-07-12T20:12:12Z")
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, ts)
}

func Test_StringToProtobufTimestamp_FailureCannotParse(t *testing.T) {
	ts, err := StringToProtobufTimestamp("2019-070:12:12Z")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "could not parse timestamp")
	assert.Nil(t, ts)
}

func Test_StringToProtobufTimestamp_FailureCannotTransformToPB(t *testing.T) {
	ts, err := StringToProtobufTimestamp("0000-07-12T20:12:12Z")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "could not transform to proto timestamp")
	assert.Nil(t, ts)
}
