/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"github.com/dgraph-io/badger"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/salesforce/sloop/pkg/sloop/test/assertex"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const someMinute = 13
const someReason = "someReason"
const someCount = 23

func Test_EventCountTableKey_OutputCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	k := NewEventCountKey(someTs, someKind, someNamespace, someName, someUid)
	assert.Equal(t, "/eventcount/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8", k.String())
}

func Test_EventCountTableKey_ParseCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	k := &EventCountKey{}
	err := k.Parse("/eventcount/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8")
	assert.Nil(t, err)
	assert.Equal(t, "001546398000", k.PartitionId)
	assert.Equal(t, someNamespace, k.Namespace)
	assert.Equal(t, someName, k.Name)
	assert.Equal(t, someUid, k.Uid)
}

func helper_update_eventcount_table(t *testing.T) (badgerwrap.DB, *ResourceEventCountsTable) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	var keys []string
	for i := 'a'; i < 'd'; i++ {
		// add keys in ascending order
		keys = append(keys, NewEventCountKey(someTs, someKind, someNamespace, someName+string(i), someUid).String())
	}
	expectedResult := &ResourceEventCounts{MapMinToEvents: make(map[int64]*EventCounts)}
	expectedResult.MapMinToEvents[someMinute] = &EventCounts{MapReasonToCount: make(map[string]int32)}
	expectedResult.MapMinToEvents[someMinute].MapReasonToCount[someReason] = someCount

	b, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	et := OpenResourceEventCountsTable()
	err = b.Update(func(txn badgerwrap.Txn) error {
		var txerr error
		for _, key := range keys {
			txerr = et.Set(txn, key, expectedResult)
			if txerr != nil {
				return txerr
			}
		}
		return nil
	})
	assert.Nil(t, err)
	return b, et
}

func Test_EventCount_PutThenGet_SameData(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	key := NewEventCountKey(someTs, someKind, someNamespace, someName, someUid).String()
	val := &ResourceEventCounts{MapMinToEvents: make(map[int64]*EventCounts)}
	val.MapMinToEvents[someMinute] = &EventCounts{MapReasonToCount: make(map[string]int32)}
	val.MapMinToEvents[someMinute].MapReasonToCount[someReason] = someCount

	b, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wt := OpenResourceEventCountsTable()

	err = b.Update(func(txn badgerwrap.Txn) error {
		txerr := wt.Set(txn, key, val)
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)

	var retval *ResourceEventCounts
	err = b.View(func(txn badgerwrap.Txn) error {
		var txerr error
		retval, txerr = wt.Get(txn, key)
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)

	assertex.ProtoEqual(t, val, retval)
}

func Test_EventCount_TestMinAndMaxKeys(t *testing.T) {
	db, rt := helper_update_eventcount_table(t)
	var minKey string
	var maxKey string
	err := db.View(func(txn badgerwrap.Txn) error {
		_, minKey = rt.GetMinKey(txn)
		_, maxKey = rt.GetMaxKey(txn)
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, "/eventcount/001546398000/somekind/somenamespace/somenamea/68510937-4ffc-11e9-8e26-1418775557c8", minKey)
	assert.Equal(t, "/eventcount/001546398000/somekind/somenamespace/somenamec/68510937-4ffc-11e9-8e26-1418775557c8", maxKey)
}

func Test_EventCount_TestGetMinMaxPartitions(t *testing.T) {
	db, rt := helper_update_eventcount_table(t)
	var minPartition string
	var maxPartition string
	var found bool
	err := db.View(func(txn badgerwrap.Txn) error {
		found, minPartition, maxPartition = rt.GetMinMaxPartitions(txn)
		return nil
	})

	assert.Nil(t, err)
	assert.True(t, found)
	assert.Equal(t, untyped.GetPartitionId(someTs), minPartition)
	assert.Equal(t, untyped.GetPartitionId(someTs), maxPartition)
}

func (_ *EventCountKey) GetTestKey() string {
	k := NewEventCountKey(someTs, "someKind", "someNamespace", "someName", "someUuid")
	return k.String()
}

func (_ *EventCountKey) GetTestValue() *ResourceEventCounts {
	return &ResourceEventCounts{}
}
