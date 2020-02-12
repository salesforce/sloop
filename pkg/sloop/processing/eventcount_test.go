/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package processing

import (
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

var (
	someEventWatchTs       = time.Date(2019, 8, 29, 21, 24, 55, 6, time.UTC)
	someEventWatchPTime, _ = ptypes.TimestampProto(someEventWatchTs)
	someMaxLookback        = time.Duration(time.Hour * 24 * 14)
	firstTimeStamp         = "2019-08-29T21:24:55Z"
	lastTimeStamp          = "2019-08-29T21:27:55Z"
	someEventPayload       = `{
  "metadata": {
    "name": "someEventName",
    "namespace": "someNamespace",
    "uid": "someEventUid"
  },
  "involvedObject": {
    "kind": "Pod",
    "namespace": "someNamespace",
    "name": "somePodName",
    "uid": "somePodUid"
  },
  "reason":"failed",
  "firstTimestamp": "[firstTimestamp]",
  "lastTimestamp":  "[lastTimestamp]",
  "count": 10,
  "type": "Warning"
}`
)

const (
	expectedEventKey    = "/eventcount/001567036800/Pod/someNamespace/somePodName/somePodUid"
	expectedEventMinKey = 1567113900
	expectedEventReason = "failed:Warning"
)

func Test_EventCountTable_NonEvent(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour * 24)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	watchRec := typed.KubeWatchResult{
		Kind:      "Pod",
		Timestamp: someEventWatchPTime,
	}

	err = tables.Db().Update(func(txn badgerwrap.Txn) error {
		return updateEventCountTable(tables, txn, &watchRec, nil, nil, someMaxLookback)
	})
	assert.Nil(t, err)

	foundKeys, err := findEventKeys(tables, 1)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(foundKeys))
}

func Test_EventCountTable_Event(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour * 24)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	addEventCount(t, tables, someEventWatchPTime, firstTimeStamp, lastTimeStamp)

	helper_dumpKeys(t, tables.Db(), "After adding event")

	foundKeys, err := findEventKeys(tables, 1)
	assert.Nil(t, err)
	assert.Equal(t, []string{expectedEventKey}, foundKeys)

	counts, err := getEventKey(db, tables.EventCountTable(), expectedEventKey)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(counts.MapMinToEvents))

	// We should have these 4 minutes all with 3 failed events

	// mapMinToEvents:<key:1567113900 value:<mapReasonToCount:<key:"failed" value:3 > > >
	// mapMinToEvents:<key:1567113960 value:<mapReasonToCount:<key:"failed" value:3 > > >
	// mapMinToEvents:<key:1567114020 value:<mapReasonToCount:<key:"failed" value:3 > > >
	// mapMinToEvents:<key:1567114080 value:<mapReasonToCount:<key:"failed" value:3 > > >

	reasonCounts := counts.MapMinToEvents[expectedEventMinKey].MapReasonToCount
	assert.Equal(t, 1, len(reasonCounts))
	assert.Equal(t, int32(4), reasonCounts[expectedEventReason])

}

func Test_EventCountTable_Event_Truncated_Before_TruncateTS(t *testing.T) {
	// This tests the following scenario
	// 1. An event is added, truncateTS is set to its start Time - 1 partition duration
	// 2. And older is added which is older than the value of truncate TS
	// Expected Behavior: The old event would be truncated.

	untyped.TestHookSetPartitionDuration(time.Hour * 24)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	eventStartTime := "2019-08-29T21:24:55Z"
	eventEndTime := "2019-08-29T21:27:55Z"
	// adding the first event which ends up adding the first partition
	addEventCount(t, tables, someEventWatchPTime, eventStartTime, eventEndTime)

	foundKeys, err := findEventKeys(tables, 6)
	assert.Nil(t, err)
	assert.Equal(t, []string{expectedEventKey}, foundKeys)
	assert.Equal(t, 1, len(foundKeys))

	helper_dumpKeys(t, tables.Db(), "After adding 1st event")

	startTSBeforeTruncateTS := "2019-08-27T21:21:55Z"
	endTSBeforeTruncateTS := "2019-08-28T21:23:55Z"

	addEventCount(t, tables, someEventWatchPTime, startTSBeforeTruncateTS, endTSBeforeTruncateTS)

	helper_dumpKeys(t, tables.Db(), "After adding event that occurred before truncateTS")

	foundKeys, err = findEventKeys(tables, 3*24)
	assert.Nil(t, err)

	// The new key is truncated as its before the start of minPartition and thus the number of keys remain 1
	assert.Equal(t, []string{expectedEventKey}, foundKeys)
	assert.Equal(t, 1, len(foundKeys))
}

func Test_EventCountTable_Events_Added_After_TruncateTS(t *testing.T) {
	// This tests the following scenario
	// 1. An event is added, truncateTS is set to its start Time - 1 partition duration
	// 2. And new event is added which is after truncateTs
	// 3. Third event is added which is also after Truncate TS
	// Expected Behavior: The new events will be added.

	untyped.TestHookSetPartitionDuration(time.Hour * 24)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	timestamp28AugustStartTime := "2019-08-28T21:24:55Z"
	timestamp28AugustEndTime := "2019-08-28T23:27:55Z"
	eventPTime28August, _ := ptypes.TimestampProto(time.Date(2019, 8, 28, 21, 24, 55, 6, time.UTC))

	// adding the first event which ends up adding the first partition
	addEventCount(t, tables, eventPTime28August, timestamp28AugustStartTime, timestamp28AugustEndTime)

	timestamp29AugustStartTime := "2019-08-29T21:24:55Z"
	timestamp29AugustEndTime := "2019-08-29T23:27:55Z"
	eventPTime29August, _ := ptypes.TimestampProto(time.Date(2019, 8, 29, 21, 24, 55, 6, time.UTC))

	// adding the second event which ends up adding the second partition
	addEventCount(t, tables, eventPTime29August, timestamp29AugustStartTime, timestamp29AugustEndTime)

	timestamp30AugustStartTime := "2019-08-30T21:21:55Z"
	timestamp30AugustEndTime := "2019-08-30T21:23:55Z"
	eventPTime30August, _ := ptypes.TimestampProto(time.Date(2019, 8, 30, 21, 24, 55, 6, time.UTC))

	addEventCount(t, tables, eventPTime30August, timestamp30AugustStartTime, timestamp30AugustEndTime)

	foundKeys, err := findEventKeys(tables, 24*3)
	assert.Nil(t, err)

	// The new keys are added as successfully
	assert.Equal(t, 3, len(foundKeys))

	helper_dumpKeys(t, tables.Db(), "After adding the three events")

	expectedNewEventKey := "/eventcount/001567123200/Pod/someNamespace/somePodName/somePodUid"

	// The new key for 30th August has been added successfully
	counts, err := getEventKey(db, tables.EventCountTable(), expectedNewEventKey)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(counts.MapMinToEvents))
}

func Test_EventCountTable_DupeEventSameResults(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour * 24)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	addEventCount(t, tables, someEventWatchPTime, firstTimeStamp, lastTimeStamp)

	helper_dumpKeys(t, tables.Db(), "After first time processing event")

	addEventCount(t, tables, someEventWatchPTime, firstTimeStamp, lastTimeStamp)

	foundKeys, err := findEventKeys(tables, 1)
	assert.Nil(t, err)

	fmt.Printf("Keys: %v\n", foundKeys)

	assert.Equal(t, []string{expectedEventKey}, foundKeys)

	counts, err := getEventKey(db, tables.EventCountTable(), expectedEventKey)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(counts.MapMinToEvents))

	reasonCounts := counts.MapMinToEvents[expectedEventMinKey].MapReasonToCount
	assert.Equal(t, 1, len(reasonCounts))
	assert.Equal(t, int32(4), reasonCounts[expectedEventReason])
}

func addEventCount(t *testing.T, tables typed.Tables, timeStamp *timestamp.Timestamp, firstTimeStamp string, lastTimeStamp string) {
	watchRec := typed.KubeWatchResult{
		Kind:      kubeextractor.EventKind,
		Timestamp: timeStamp,
		Payload:   get_event_pay_load(firstTimeStamp, lastTimeStamp),
	}

	resourceMetadata, err := kubeextractor.ExtractMetadata(watchRec.Payload)
	assert.Nil(t, err)
	involvedObject, err := kubeextractor.ExtractInvolvedObject(watchRec.Payload)
	assert.Nil(t, err)

	err = tables.Db().Update(func(txn badgerwrap.Txn) error {
		// For dedupe to work we need a record written to the watch table
		err2 := updateEventCountTable(tables, txn, &watchRec, &resourceMetadata, &involvedObject, someMaxLookback)
		if err2 != nil {
			return err2
		}

		kubeMetadata, err := kubeextractor.ExtractMetadata(watchRec.Payload)
		assert.Nil(t, err)
		err2 = updateKubeWatchTable(tables, txn, &watchRec, &kubeMetadata, false)
		return err2
	})
	assert.Nil(t, err)

}

func findEventKeys(tables typed.Tables, numberOfHours int64) ([]string, error) {

	var foundKeys []string
	err := tables.Db().View(func(txn badgerwrap.Txn) error {
		ret, _, err2 := tables.EventCountTable().RangeRead(txn, nil, func(s string) bool { return true }, nil, someEventWatchTs.Add(-time.Duration(numberOfHours)*time.Hour), someEventWatchTs.Add(time.Duration(numberOfHours)*time.Hour))
		if err2 != nil {
			return err2
		}
		for k, _ := range ret {
			foundKeys = append(foundKeys, k.String())
		}
		return nil
	})
	return foundKeys, err
}

func getEventKey(db badgerwrap.DB, table *typed.ResourceEventCountsTable, key string) (*typed.ResourceEventCounts, error) {
	var val *typed.ResourceEventCounts
	err := db.View(func(txn badgerwrap.Txn) error {
		v, err := table.Get(txn, key)
		if err != nil {
			return err
		}
		val = v
		return nil
	})
	return val, err
}

func helper_dumpKeys(t *testing.T, db badgerwrap.DB, message string) {
	fmt.Printf("%v\n", message)
	err := db.View(func(txn badgerwrap.Txn) error {
		itr := txn.NewIterator(badger.DefaultIteratorOptions)
		for itr.Rewind(); itr.Valid(); itr.Next() {
			fmt.Printf("KEY %v\n", string(itr.Item().Key()))
		}
		return nil
	})
	assert.Nil(t, err)
}

func get_event_pay_load(firstTimeStamp string, lastTimeStamp string) string {
	payLoad := strings.ReplaceAll(someEventPayload, "[firstTimestamp]", firstTimeStamp)
	return strings.ReplaceAll(payLoad, "[lastTimestamp]", lastTimeStamp)
}

func Test_distributeValue(t *testing.T) {
	assert.Equal(t, []int{}, distributeValue(8, 0))
	assert.Equal(t, []int{8}, distributeValue(8, 1))
	assert.Equal(t, []int{2, 2, 2}, distributeValue(6, 3))
	assert.Equal(t, []int{3, 2, 2}, distributeValue(7, 3))
	assert.Equal(t, []int{3, 3, 2}, distributeValue(8, 3))
	assert.Equal(t, []int{3, 3, 3}, distributeValue(9, 3))
}

var someEventTs1 = time.Date(2019, 8, 29, 21, 24, 55, 6, time.UTC)
var someEventTs2 = someEventTs1.Add(time.Minute)
var someEventTs3 = someEventTs1.Add(2 * time.Minute)
var someEventTs4 = someEventTs1.Add(3 * time.Minute)

func Test_computeEventsDiff_NoOldEvent(t *testing.T) {
	newEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs1,
		LastTimestamp:  someEventTs1,
		Count:          123,
	}
	t1, t2, count := computeEventsDiff(nil, newEventInfo)
	assert.Equal(t, 123, count)
	assert.Equal(t, someEventTs1, t1)
	assert.Equal(t, someEventTs1, t2)
}

func Test_computeEventsDiff_DupeEvent(t *testing.T) {
	newEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs1,
		LastTimestamp:  someEventTs1,
		Count:          123,
	}
	t1, t2, count := computeEventsDiff(newEventInfo, newEventInfo)
	assert.Equal(t, 0, count)
	assert.Equal(t, time.Time{}, t1)
	assert.Equal(t, time.Time{}, t2)
}

func Test_computeEventsDiff_DupeEventWithDiffCount(t *testing.T) {
	prevEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs1,
		LastTimestamp:  someEventTs1,
		Count:          122,
	}
	newEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs1,
		LastTimestamp:  someEventTs1,
		Count:          123,
	}
	t1, t2, count := computeEventsDiff(prevEventInfo, newEventInfo)
	assert.Equal(t, 0, count)
	assert.Equal(t, time.Time{}, t1)
	assert.Equal(t, time.Time{}, t2)
}

func Test_computeEventsDiff_GotAnOldEvent(t *testing.T) {
	oldEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs1,
		LastTimestamp:  someEventTs3,
		Count:          10,
	}
	newEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs1,
		LastTimestamp:  someEventTs2,
		Count:          13,
	}
	t1, t2, count := computeEventsDiff(oldEventInfo, newEventInfo)
	assert.Equal(t, 0, count)
	assert.Equal(t, time.Time{}, t1)
	assert.Equal(t, time.Time{}, t2)
}

func Test_computeEventsDiff_NewEventsWithMoreCount(t *testing.T) {
	oldEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs1,
		LastTimestamp:  someEventTs2,
		Count:          10,
	}
	newEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs1,
		LastTimestamp:  someEventTs3,
		Count:          13,
	}
	t1, t2, count := computeEventsDiff(oldEventInfo, newEventInfo)
	assert.Equal(t, 3, count)
	assert.Equal(t, someEventTs2, t1)
	assert.Equal(t, someEventTs3, t2)
}

func Test_computeEventsDiff_PartiallyOverlapping(t *testing.T) {
	oldEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs1,
		LastTimestamp:  someEventTs3,
		Count:          10,
	}
	newEventInfo := &kubeextractor.EventInfo{
		FirstTimestamp: someEventTs2,
		LastTimestamp:  someEventTs4,
		Count:          6,
	}
	t1, t2, count := computeEventsDiff(oldEventInfo, newEventInfo)
	assert.Equal(t, 1, count)
	assert.Equal(t, someEventTs3, t1)
	assert.Equal(t, someEventTs4, t2)
}

func Test_adjustForMaxLookback_ShortEventNoChange(t *testing.T) {
	first, last, count := adjustForMaxLookback(someEventTs3, someEventTs4, 100, someEventTs1)
	assert.Equal(t, someEventTs3, first)
	assert.Equal(t, someEventTs4, last)
	assert.Equal(t, 100, count)
}

func Test_adjustForMaxLookback_LongEventGetsTruncated(t *testing.T) {
	first, last, count := adjustForMaxLookback(someEventTs1, someEventTs4, 1000, someEventTs3)
	assert.Equal(t, someEventTs3, first)
	assert.Equal(t, someEventTs4, last)
	assert.Equal(t, 333, count)
}
