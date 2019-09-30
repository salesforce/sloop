/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
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
