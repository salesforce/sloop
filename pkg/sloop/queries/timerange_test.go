/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var someQueryStartTs = time.Date(2019, 3, 1, 3, 4, 0, 0, time.UTC)
var someQueryEndTs = someQueryStartTs.Add(time.Hour)
var rightBeforeStartTs = someQueryStartTs.Add(-1 * time.Minute)
var rightAfterStartTs = someQueryStartTs.Add(time.Minute)
var rigthAfterEndTs = someQueryEndTs.Add(time.Minute)
var someMaxLookBack = time.Duration(24) * time.Hour

func Test_timeRangeTable(t *testing.T) {
	var rangeTests = []struct {
		lookbackStr   string
		startStr      string
		endStr        string
		shouldFail    bool
		expectedStart time.Time
		expectedEnd   time.Time
	}{
		// Valid Cases
		{"1h", "", "", false, someQueryEndTs.Add(-1 * time.Hour), someQueryEndTs},
		{"", fmt.Sprintf("%v", someQueryEndTs.Add(time.Minute*-45).UTC().Unix()), fmt.Sprintf("%v", someQueryEndTs.Add(time.Minute*-15).UTC().Unix()), false, someQueryEndTs.Add(time.Minute * -45), someQueryEndTs.Add(time.Minute * -15)},

		// Too short, gets adjusted
		{"0h", "", "", false, someQueryEndTs.Add(-1 * minLookback), someQueryEndTs},
		{"", fmt.Sprintf("%v", someQueryEndTs.UTC().Unix()), fmt.Sprintf("%v", someQueryEndTs.UTC().Unix()), false, someQueryEndTs.Add(-1 * minLookback), someQueryEndTs},
		// Too long, gets adjusted
		{"1000h", "", "", false, someQueryEndTs.Add(-1 * someMaxLookBack), someQueryEndTs},
		{"", fmt.Sprintf("%v", someQueryEndTs.Add(-1000*time.Hour).UTC().Unix()), fmt.Sprintf("%v", someQueryEndTs.UTC().Unix()), false, someQueryEndTs.Add(-1 * someMaxLookBack), someQueryEndTs},
		// Ends in the future
		{"", fmt.Sprintf("%v", someQueryEndTs.Add(time.Minute*-15).UTC().Unix()), fmt.Sprintf("%v", someQueryEndTs.Add(time.Minute*15).UTC().Unix()), false, someQueryEndTs.Add(time.Minute * -30), someQueryEndTs},

		// Missing or too many inputs
		{"", "", "", true, time.Time{}, time.Time{}},
		{"", "123", "", true, time.Time{}, time.Time{}},
		{"", "", "123", true, time.Time{}, time.Time{}},
		{"1h", "123", "123", true, time.Time{}, time.Time{}},
		// Invalid times
		{"abc", "", "", true, time.Time{}, time.Time{}},
		{"", "abc", "123", true, time.Time{}, time.Time{}},
		{"", "123", "abc", true, time.Time{}, time.Time{}},
	}
	for _, thisRange := range rangeTests {
		t.Run(fmt.Sprintf("%+v", thisRange), func(t *testing.T) {
			paramMap := make(map[string][]string)
			paramMap[LookbackParam] = []string{thisRange.lookbackStr}
			paramMap[StartTimeParam] = []string{thisRange.startStr}
			paramMap[EndTimeParam] = []string{thisRange.endStr}
			actStart, actEnd, err := computeTimeRangeInternal(paramMap, someQueryEndTs, someMaxLookBack)
			if thisRange.shouldFail {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, thisRange.expectedStart, actStart)
				assert.Equal(t, thisRange.expectedEnd, actEnd)
			}
		})
	}
}

func Test_timeFilterResSumValue_OutsideRangeDrop(t *testing.T) {
	resVal := typed.ResourceSummary{}
	resVal.CreateTime, _ = ptypes.TimestampProto(rightBeforeStartTs)
	resVal.LastSeen, _ = ptypes.TimestampProto(rightBeforeStartTs)
	keep, err := timeFilterResSumValue(&resVal, someQueryStartTs, someQueryEndTs)
	assert.Nil(t, err)
	assert.False(t, keep)
}

func Test_timeFilterResSumValue_InsideRangeKeepAndNoChange(t *testing.T) {
	resVal := typed.ResourceSummary{}
	resVal.CreateTime, _ = ptypes.TimestampProto(rightAfterStartTs)
	resVal.LastSeen, _ = ptypes.TimestampProto(rightAfterStartTs)

	keep, err := timeFilterResSumValue(&resVal, someQueryStartTs, someQueryEndTs)
	assert.Nil(t, err)
	assert.True(t, keep)

	afterCreateTime, _ := ptypes.Timestamp(resVal.CreateTime)
	afterLastSeenTime, _ := ptypes.Timestamp(resVal.LastSeen)
	assert.Equal(t, rightAfterStartTs, afterCreateTime)
	assert.Equal(t, rightAfterStartTs, afterLastSeenTime)
}

func Test_timeFilterResSumValue_StartsBeforeChangeIsClipped(t *testing.T) {
	resVal := typed.ResourceSummary{}
	resVal.CreateTime, _ = ptypes.TimestampProto(rightBeforeStartTs)
	resVal.LastSeen, _ = ptypes.TimestampProto(rightAfterStartTs)
	keep, err := timeFilterResSumValue(&resVal, someQueryStartTs, someQueryEndTs)
	assert.Nil(t, err)
	assert.True(t, keep)

	// CreateTime should match query start time
	afterCreateTime, _ := ptypes.Timestamp(resVal.CreateTime)
	afterLastSeenTime, _ := ptypes.Timestamp(resVal.LastSeen)
	assert.Equal(t, someQueryStartTs, afterCreateTime)
	assert.Equal(t, rightAfterStartTs, afterLastSeenTime)
}

func Test_timeFilterResSumValue_EndsAfterChangeIsClipped(t *testing.T) {
	resVal := typed.ResourceSummary{}
	resVal.CreateTime, _ = ptypes.TimestampProto(rightAfterStartTs)
	resVal.LastSeen, _ = ptypes.TimestampProto(rigthAfterEndTs)
	keep, err := timeFilterResSumValue(&resVal, someQueryStartTs, someQueryEndTs)
	assert.Nil(t, err)
	assert.True(t, keep)

	// CreateTime should match query start time
	afterCreateTime, _ := ptypes.Timestamp(resVal.CreateTime)
	afterLastSeenTime, _ := ptypes.Timestamp(resVal.LastSeen)
	assert.Equal(t, rightAfterStartTs, afterCreateTime)
	assert.Equal(t, someQueryEndTs, afterLastSeenTime)
}

func Test_timeFilterResSumValue_ExtendsOnBothSizedIsClipped(t *testing.T) {
	resVal := typed.ResourceSummary{}
	resVal.CreateTime, _ = ptypes.TimestampProto(rightBeforeStartTs)
	resVal.LastSeen, _ = ptypes.TimestampProto(rigthAfterEndTs)
	keep, err := timeFilterResSumValue(&resVal, someQueryStartTs, someQueryEndTs)
	assert.Nil(t, err)
	assert.True(t, keep)

	// CreateTime should match query start time
	afterCreateTime, _ := ptypes.Timestamp(resVal.CreateTime)
	afterLastSeenTime, _ := ptypes.Timestamp(resVal.LastSeen)
	assert.Equal(t, someQueryStartTs, afterCreateTime)
	assert.Equal(t, someQueryEndTs, afterLastSeenTime)
}

func Test_timeFilterWatchActivityOccurrences(t *testing.T) {
	occurrences := []int64{rightBeforeStartTs.Unix(), someQueryStartTs.Unix(), rightAfterStartTs.Unix(), someQueryEndTs.Unix(), rigthAfterEndTs.Unix()}
	filtered := timeFilterWatchActivityOccurrences(occurrences, someQueryStartTs, someQueryEndTs)
	assert.Len(t, filtered, 3)
	for _, item := range filtered {
		assert.True(t, item == someQueryStartTs.Unix() || item == rightAfterStartTs.Unix() || item == someQueryEndTs.Unix())
	}
}

func Test_timeFilterWatchActivityMap(t *testing.T) {
	activityMap := make(map[typed.WatchActivityKey]*typed.WatchActivity)
	activityMap[typed.WatchActivityKey{Name: "before1"}] = &typed.WatchActivity{ChangedAt: []int64{rightBeforeStartTs.Unix()}}
	activityMap[typed.WatchActivityKey{Name: "before2"}] = &typed.WatchActivity{NoChangeAt: []int64{rightBeforeStartTs.Unix()}}
	activityMap[typed.WatchActivityKey{Name: "during1"}] = &typed.WatchActivity{ChangedAt: []int64{rightAfterStartTs.Unix()}}
	activityMap[typed.WatchActivityKey{Name: "during2"}] = &typed.WatchActivity{NoChangeAt: []int64{rightAfterStartTs.Unix()}}
	activityMap[typed.WatchActivityKey{Name: "after1"}] = &typed.WatchActivity{ChangedAt: []int64{rigthAfterEndTs.Unix()}}
	activityMap[typed.WatchActivityKey{Name: "after2"}] = &typed.WatchActivity{NoChangeAt: []int64{rigthAfterEndTs.Unix()}}

	timeFilterWatchActivityMap(activityMap, someQueryStartTs, someQueryEndTs)
	assert.Len(t, activityMap, 2)
	assert.Contains(t, activityMap, typed.WatchActivityKey{Name: "during1"})
	assert.Contains(t, activityMap, typed.WatchActivityKey{Name: "during2"})
}
