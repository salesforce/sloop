/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	badger "github.com/dgraph-io/badger/v2"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
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
	db, rt := helper_update_ResourceEventCountsTable(t, (&EventCountKey{}).SetTestKeys(), &ResourceEventCounts{})
	var minKey string
	var maxKey string
	err := db.View(func(txn badgerwrap.Txn) error {
		_, minKey = rt.GetMinKey(txn)
		_, maxKey = rt.GetMaxKey(txn)
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, "/eventcount/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8", minKey)
	assert.Equal(t, "/eventcount/001546405200/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8c", maxKey)
}

func Test_EventCount_TestGetMinMaxPartitions(t *testing.T) {
	db, rt := helper_update_ResourceEventCountsTable(t, (&EventCountKey{}).SetTestKeys(), &ResourceEventCounts{})
	var minPartition string
	var maxPartition string
	var found bool
	err := db.View(func(txn badgerwrap.Txn) error {
		found, minPartition, maxPartition = rt.GetMinMaxPartitions(txn)
		return nil
	})

	assert.Nil(t, err)
	assert.True(t, found)
	assert.Equal(t, someMinPartition, minPartition)
	assert.Equal(t, someMaxPartition, maxPartition)
}

func Test_EventCount_getLastMatchingKeyInPartition_FoundInPreviousPartition(t *testing.T) {
	db, wt := helper_update_ResourceEventCountsTable(t, (&EventCountKey{}).SetTestKeys(), (&EventCountKey{}).SetTestValue())
	var keyRes *EventCountKey
	var err1 error
	var found bool
	curKey := NewEventCountKey(someMaxTs, someKind, someNamespace, someName, someUid)
	keyComparator := NewEventCountKeyComparator(someKind, someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMiddlePartition, curKey, keyComparator)
		return err1
	})
	assert.True(t, found)
	expectedKey := NewEventCountKey(someMiddleTs, someKind, someNamespace, someName, someUid+"b")
	assert.Equal(t, expectedKey, keyRes)
	assert.Nil(t, err)
}

func Test_EventCount_getLastMatchingKeyInPartition_FoundInSamePartition(t *testing.T) {
	db, wt := helper_update_ResourceEventCountsTable(t, (&EventCountKey{}).SetTestKeys(), (&EventCountKey{}).SetTestValue())
	var keyRes *EventCountKey
	var err1 error
	var found bool
	curKey := NewEventCountKey(someTs, someKind, someNamespace, someName, someUid)
	keyComparator := NewEventCountKeyComparator(someKind, someNamespace, someName, someUid+"a")
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMinPartition, curKey, keyComparator)
		return err1
	})

	assert.True(t, found)
	expectedKey := NewEventCountKey(someTs, someKind, someNamespace, someName, someUid+"a")
	assert.Equal(t, expectedKey, keyRes)
	assert.Nil(t, err)
}

func Test_EventCount_getLastMatchingKeyInPartition_SameKeySearch(t *testing.T) {
	db, wt := helper_update_ResourceEventCountsTable(t, (&EventCountKey{}).SetTestKeys(), (&EventCountKey{}).SetTestValue())
	var keyRes *EventCountKey
	var err1 error
	var found bool
	curKey := NewEventCountKey(someTs, someKind, someNamespace, someName, someUid+"a")
	keyComparator := NewEventCountKeyComparator(someKind, someNamespace, someName, someUid+"a")
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMinPartition, curKey, keyComparator)
		return err1
	})

	assert.False(t, found)
	assert.Equal(t, &EventCountKey{}, keyRes)
	assert.Nil(t, err)
}

func Test_EventCount_getLastMatchingKeyInPartition_NotFound(t *testing.T) {
	db, wt := helper_update_ResourceEventCountsTable(t, (&EventCountKey{}).SetTestKeys(), (&EventCountKey{}).SetTestValue())
	var keyRes *EventCountKey
	var err1 error
	var found bool
	curKey := NewEventCountKey(someMaxTs, someKind, someNamespace, someName, someUid)
	keyComparator := NewEventCountKeyComparator(someKind, someNamespace, someName, someUid+"dfd")
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMinPartition, curKey, keyComparator)
		return err1
	})

	assert.False(t, found)
	assert.Equal(t, &EventCountKey{}, keyRes)
	assert.Nil(t, err)
}

func (*EventCountKey) GetTestKey() string {
	k := NewEventCountKey(someTs, "someKind", "someNamespace", "someName", "someUuid")
	return k.String()
}

func (*EventCountKey) GetTestValue() *ResourceEventCounts {
	return &ResourceEventCounts{}
}

func (*EventCountKey) SetTestKeys() []string {
	untyped.TestHookSetPartitionDuration(time.Hour)
	var keys []string
	i := 'a'
	for curTime := someTs; !curTime.After(someMaxTs); curTime = curTime.Add(untyped.GetPartitionDuration()) {
		// add keys in ascending order
		keys = append(keys, NewEventCountKey(curTime, someKind, someNamespace, someName, someUid).String())
		keys = append(keys, NewEventCountKey(curTime, someKind, someNamespace, someName, someUid+string(i)).String())
		i++
	}
	return keys
}

func (*EventCountKey) SetTestValue() *ResourceEventCounts {
	return &ResourceEventCounts{}
}

func helper_testKeys() []string {
	return []string{
		// someMaxTs partition
		"/eventcount/001546405200/Pod/user-j/sync-123/sam-partition-testdata",
		"/eventcount/001546405200/Pod/user-j/sync-123/sam-partition-test",

		"/eventcount/001546405200/somekind/somenamespace/somename/previous-partition-test",
		"/eventcount/001546405200/user-a/somenamespace/somename/478-aaa",
		"/eventcount/001546405200/user-b/somenamespace/somename/477-bbb",
		"/eventcount/001546405200/somekind/somenamespacetest/somename/skipped-partition",

		// someMiddleTs partition
		"/eventcount/001546401600/somekind/somenamespace/somename/previous-partition-test",
		"/eventcount/001546401600/somekind/somenamespace/somename/cde-4ffc-11e9-8e26-123",

		// someTs partition
		"/eventcount/001546398000/somekind/somenamespace/somename/previous-partition-test",
		"/eventcount/001546398000/somekind123/somenamespace/somename/68510937-4ffc-11e9-8e26-123",
		"/eventcount/001546398000/somekind/somenamespacetest/somename/skipped-partition",

		"/eventcount/001546398000/somekindaaa/somenamespaceaaa/somenameaaa/aaaa",
	}
}

func Test_EventCount_GetPreviousKey_Success(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	db, wt := helper_update_ResourceEventCountsTable(t, helper_testKeys(), (&EventCountKey{}).SetTestValue())
	var partRes *EventCountKey
	var err1 error

	// testing in the same partition
	curKey := NewEventCountKey(someMaxTs, kubeextractor.PodKind, "user-j", "sync-123", "sam-partition-testdata")
	keyComparator := NewEventCountKeyComparator(kubeextractor.PodKind, "user-j", "sync-123", "")
	err := db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComparator)
		return err1
	})
	assert.Nil(t, err)
	expectedKey := NewEventCountKey(someMaxTs, kubeextractor.PodKind, "user-j", "sync-123", "sam-partition-test")
	assert.Equal(t, expectedKey, partRes)

	// testing in the previous partition
	curKey = NewEventCountKey(someMaxTs, someKind, someNamespace, someName, "previous-partition-test")
	keyComparator = NewEventCountKeyComparator(someKind, someNamespace, someName, "previous-partition-test")
	err = db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComparator)
		return err1
	})
	assert.Nil(t, err)
	expectedKey = NewEventCountKey(someMiddleTs, someKind, someNamespace, someName, "previous-partition-test")
	assert.Equal(t, expectedKey, partRes)

	// testing in skipped partition
	curKey = NewEventCountKey(someMaxTs, someKind, "somenamespacetest", someName, "skipped-partition")
	keyComparator = NewEventCountKeyComparator(someKind, "somenamespacetest", someName, "skipped-partition")
	err = db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComparator)
		return err1
	})
	assert.Nil(t, err)
	expectedKey = NewEventCountKey(someTs, someKind, "somenamespacetest", someName, "skipped-partition")
	assert.Equal(t, expectedKey, partRes)
}

func Test_EventCount_GetPreviousKey_Fail(t *testing.T) {
	db, wt := helper_update_ResourceEventCountsTable(t, (&EventCountKey{}).SetTestKeys(), (&EventCountKey{}).SetTestValue())
	var partRes *EventCountKey
	var err1 error
	curKey := NewEventCountKey(someMaxTs, someKind, someNamespace, someName, someUid)
	keyComparator := NewEventCountKeyComparator(someKind+"b", someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComparator)
		return err1
	})
	assert.NotNil(t, err)
	assert.Equal(t, &EventCountKey{}, partRes)
}
