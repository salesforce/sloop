/**
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package untyped

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var someTs = time.Date(2019, 1, 2, 3, 4, 5, 6, time.UTC)
var someTsRoundedHour = time.Date(2019, 1, 2, 3, 0, 0, 0, time.UTC)
var someTsRoundedDay = time.Date(2019, 1, 2, 0, 0, 0, 0, time.UTC)

func Test_PartitionsRoundTrip_Hour(t *testing.T) {
	TestHookSetPartitionDuration(time.Hour)
	partStr := GetPartitionId(someTs)
	minTs, maxTs, err := GetTimeRangeForPartition(partStr)
	assert.Nil(t, err)
	assert.Equal(t, someTsRoundedHour, minTs)
	assert.Equal(t, someTsRoundedHour.Add(time.Hour), maxTs)
}

func Test_PartitionsRoundTrip_Day(t *testing.T) {
	TestHookSetPartitionDuration(24 * time.Hour)
	partStr := GetPartitionId(someTs)
	minTs, maxTs, err := GetTimeRangeForPartition(partStr)
	assert.Nil(t, err)
	assert.Equal(t, someTsRoundedDay, minTs)
	assert.Equal(t, someTsRoundedDay.Add(24*time.Hour), maxTs)
}
