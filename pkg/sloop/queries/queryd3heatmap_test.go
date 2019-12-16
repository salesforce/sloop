/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/salesforce/sloop/pkg/sloop/test/assertex"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
	"time"
)

const (
	kindPod       = "Pod"
	someNamespace = "somens"
	someName      = "somename"
	someUid       = "someuid"
)

var someHeatMapQueryStart = time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC)
var heatMapPartStart = someHeatMapQueryStart
var someResSumTs = someHeatMapQueryStart.Add(time.Minute)
var firstSeenTs = someHeatMapQueryStart.Add(2 * time.Minute)
var events1Ts = someHeatMapQueryStart.Add(7 * time.Minute)
var events2Ts = someHeatMapQueryStart.Add(28 * time.Minute)
var lastSeenTs = someHeatMapQueryStart.Add(50 * time.Minute)
var someHeatMapQueryEnd = someHeatMapQueryStart.Add(60 * time.Minute)

func helper_AddResSum(t *testing.T, tables typed.Tables) {
	someResSumKey := typed.NewResourceSummaryKey(someResSumTs, kindPod, someNamespace, someName, someUid)

	CreatePts, _ := ptypes.TimestampProto(firstSeenTs)
	LastSeenPts, _ := ptypes.TimestampProto(lastSeenTs)
	someResSumVal := typed.ResourceSummary{
		CreateTime: CreatePts,
		LastSeen:   LastSeenPts,
	}

	err := tables.Db().Update(func(txn badgerwrap.Txn) error {
		return tables.ResourceSummaryTable().Set(txn, someResSumKey.String(), &someResSumVal)
	})
	assert.Nil(t, err)
}

func helper_AddEventSum(t *testing.T, tables typed.Tables) {
	someEventKey := typed.NewEventCountKey(someResSumTs, kindPod, someNamespace, someName, someUid)

	firstEventsMin := events1Ts.Unix()
	assert.Equal(t, int64(1551398820), firstEventsMin)

	secondEventsMin := events2Ts.Unix()
	assert.Equal(t, int64(1551400080), secondEventsMin)

	someEventValue := typed.ResourceEventCounts{
		MapMinToEvents: map[int64]*typed.EventCounts{
			firstEventsMin: {
				MapReasonToCount: map[string]int32{
					"ImagePullError":      1,
					"LivenessProveFailed": 2,
				},
			},
			secondEventsMin: {
				MapReasonToCount: map[string]int32{
					"ContainerCreated": 3,
				},
			},
		},
	}

	err := tables.Db().Update(func(txn badgerwrap.Txn) error {
		return tables.EventCountTable().Set(txn, someEventKey.String(), &someEventValue)
	})
	assert.Nil(t, err)
}

func helper_AddWatchActivity(t *testing.T, tables typed.Tables) {
	someWatchActivityKey := typed.NewWatchActivityKey(untyped.GetPartitionId(someResSumTs), kindPod, someNamespace, someName, someUid)

	firstActivity := events1Ts.Unix()
	assert.Equal(t, int64(1551398820), firstActivity)

	secondActivity := events2Ts.Unix()
	assert.Equal(t, int64(1551400080), secondActivity)

	someWatchActivity := &typed.WatchActivity{
		NoChangeAt: []int64{firstActivity},
		ChangedAt:  []int64{secondActivity},
	}
	err := tables.Db().Update(func(txn badgerwrap.Txn) error {
		return tables.WatchActivityTable().Set(txn, someWatchActivityKey.String(), someWatchActivity)
	})
	assert.Nil(t, err)
}

func helper_UrlValues() url.Values {
	return map[string][]string{
		NamespaceParam: []string{AllNamespaces},
		KindParam:      []string{AllKinds},
	}
}

func Test_EventHeatMap3_SimpleTestWithOneDeployment(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour * 24)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	helper_AddResSum(t, tables)

	resultJsonBytes, err := EventHeatMap3Query(helper_UrlValues(), tables, someHeatMapQueryStart, someHeatMapQueryEnd, someRequestId)
	assert.Nil(t, err)
	expectedJson := `{
 "view_options": {
  "sort": ""
 },
 "rows": [
  {
   "text": "somename",
   "duration": 3480,
   "kind": "Pod",
   "namespace": "somens",
   "overlays": [],
   "changedat": null,
   "nochangeat": null,
   "start_date": 1551398520,
   "end_date": 1551402000
  }
 ]
}`
	assertex.JsonEqual(t, expectedJson, string(resultJsonBytes))
}

func Test_EventHeatMap3_OneDeploymentAnd3Events(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour * 24)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	helper_AddResSum(t, tables)
	helper_AddEventSum(t, tables)
	helper_AddWatchActivity(t, tables)

	resultJsonBytes, err := EventHeatMap3Query(helper_UrlValues(), tables, someHeatMapQueryStart, someHeatMapQueryEnd, someRequestId)
	assert.Nil(t, err)
	expectedJson := `{
 "view_options": {
  "sort": ""
 },
 "rows": [
  {
   "text": "somename",
   "duration": 3480,
   "kind": "Pod",
   "namespace": "somens",
   "overlays": [
    {
     "text": "ImagePullError:1 LivenessProveFailed:2",
     "start_date": 1551398820,
     "duration": 60,
     "end_date": 1551398880
    },
    {
     "text": "ContainerCreated:3",
     "start_date": 1551400080,
     "duration": 60,
     "end_date": 1551400140
    }
   ],
   "changedat": [
    1551400080
   ],
   "nochangeat": [
    1551398820
   ],
   "start_date": 1551398520,
   "end_date": 1551402000
  }
 ]
}`

	assertex.JsonEqual(t, expectedJson, string(resultJsonBytes))
}

var someAdjQueryEndTime = time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC)
var someAdjLastSeenOld = someAdjQueryEndTime.Add(-5 * time.Hour)
var someAdjLastSeenRecent = someAdjQueryEndTime.Add(-5 * time.Minute)
var someResyncDuration = time.Duration(30) * time.Minute

func Test_adjustLastSeenTime_TooOldNoChange(t *testing.T) {
	lastOld, _ := ptypes.TimestampProto(someAdjLastSeenOld)

	resSum := &typed.ResourceSummary{
		LastSeen: lastOld,
	}
	err := adjustLastSeenTime(resSum, someAdjQueryEndTime, someResyncDuration)
	assert.Nil(t, err)

	assert.Equal(t, lastOld, resSum.LastSeen)
}

func Test_adjustLastSeenTime_RecentChanged(t *testing.T) {
	lastOld, _ := ptypes.TimestampProto(someAdjLastSeenRecent)
	queryEnd, _ := ptypes.TimestampProto(someAdjQueryEndTime)

	resSum := &typed.ResourceSummary{
		LastSeen: lastOld,
	}
	err := adjustLastSeenTime(resSum, someAdjQueryEndTime, someResyncDuration)
	assert.Nil(t, err)

	assert.Equal(t, queryEnd, resSum.LastSeen)
}

func Test_adjustLastSeenTime_RecentButDeletedNotChanged(t *testing.T) {
	lastOld, _ := ptypes.TimestampProto(someAdjLastSeenRecent)

	resSum := &typed.ResourceSummary{
		LastSeen:     lastOld,
		DeletedAtEnd: true,
	}
	err := adjustLastSeenTime(resSum, someAdjQueryEndTime, someResyncDuration)
	assert.Nil(t, err)

	assert.Equal(t, lastOld, resSum.LastSeen)
}
