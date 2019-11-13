/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var someTs = time.Date(2019, 1, 2, 3, 4, 5, 6, time.UTC)
var someMiddleTs = time.Date(2019, 1, 2, 4, 4, 5, 6, time.UTC)
var someMaxTs = time.Date(2019, 1, 2, 5, 4, 5, 6, time.UTC)
var zeroData time.Time

const someKind = "somekind"
const someNamespace = "somenamespace"
const someName = "somename"
const someMinPartition = "001546398000"
const someMiddlePartition = "001546401600"
const someMaxPartition = "001546405200"

func Test_WatchTableKey_OutputCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	k := NewWatchTableKey(partitionId, someKind, someNamespace, someName, someTs)
	assert.Equal(t, "/watch/001546398000/somekind/somenamespace/somename/1546398245000000006", k.String())
}

func Test_WatchTableKey_ParseCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	k := &WatchTableKey{}
	err := k.Parse("/watch/001546398000/somekind/somenamespace/somename/1546398245000000006")
	assert.Nil(t, err)
	assert.Equal(t, someMinPartition, k.PartitionId)
	assert.Equal(t, someNamespace, k.Namespace)
	assert.Equal(t, someName, k.Name)
	assert.Equal(t, someTs, k.Timestamp)
}

func Test_WatchTableKey_ValidateWorks(t *testing.T) {
	testKey := "/watch/001562961600/ReplicaSet/mesh-control-plane/istio-pilot-56f7d9848/1562963507608345756"
	assert.Nil(t, (&WatchTableKey{}).ValidateKey(testKey))
}

func Test_WatchTable_PutThenGet_SameData(t *testing.T) {
	db, wt := helper_update_KubeWatchResultTable(t, (&WatchTableKey{}).SetTestKeys(), (&WatchTableKey{}).SetTestValue())

	var retval *KubeWatchResult
	err := db.View(func(txn badgerwrap.Txn) error {
		var txerr error
		retval, txerr = wt.Get(txn, "/watch/001546398000/somekind/somenamespace/somename/1546398245000000006")
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, someKind, retval.Kind)
}

func Test_WatchTable_TestMinAndMaxKeys(t *testing.T) {
	db, wt := helper_update_KubeWatchResultTable(t, (&WatchTableKey{}).SetTestKeys(), (&WatchTableKey{}).SetTestValue())
	var minKey string
	var maxKey string
	err := db.View(func(txn badgerwrap.Txn) error {
		_, minKey = wt.GetMinKey(txn)
		_, maxKey = wt.GetMaxKey(txn)
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, "/watch/001546398000/somekind/somenamespace/somename/1546380245000000006", minKey)
	assert.Equal(t, "/watch/001546405200/somekind/somenamespace/somename/1546398245000000006", maxKey)
}

func Test_WatchTable_TestGetMinMaxPartitions(t *testing.T) {
	db, wt := helper_update_KubeWatchResultTable(t, (&WatchTableKey{}).SetTestKeys(), (&WatchTableKey{}).SetTestValue())
	var minPartition string
	var maxPartition string
	var found bool
	err := db.View(func(txn badgerwrap.Txn) error {
		found, minPartition, maxPartition = wt.GetMinMaxPartitions(txn)
		return nil
	})

	assert.Nil(t, err)
	assert.True(t, found)
	assert.Equal(t, someMinPartition, minPartition)
	assert.Equal(t, someMaxPartition, maxPartition)
}

func Test_getLastMatchingKeyInPartition_FoundInPreviousPartition(t *testing.T) {
	db, wt := helper_update_KubeWatchResultTable(t, (&WatchTableKey{}).SetTestKeys(), (&WatchTableKey{}).SetTestValue())
	var keyRes *WatchTableKey
	var err1 error
	var found bool
	curKey := NewWatchTableKey(someMaxPartition, someKind, someNamespace, someName, someTs)
	keyComparator := NewWatchTableKeyComparator(someKind, someNamespace, someName, zeroData)

	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMiddlePartition, curKey, keyComparator)
		return err1
	})
	assert.True(t, found)
	expectedKey := NewWatchTableKey(someMiddlePartition, someKind, someNamespace, someName, someTs)
	assert.Equal(t, expectedKey, keyRes)
	assert.Nil(t, err)
}

func Test_getLastMatchingKeyInPartition_FoundInSamePartition(t *testing.T) {
	db, wt := helper_update_KubeWatchResultTable(t, (&WatchTableKey{}).SetTestKeys(), (&WatchTableKey{}).SetTestValue())
	var keyRes *WatchTableKey
	var err1 error
	var found bool
	curKey := NewWatchTableKey(someMaxPartition, someKind, someNamespace, someName, someTs)
	keyComparator := NewWatchTableKeyComparator(someKind, someNamespace, someName, zeroData)
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMaxPartition, curKey, keyComparator)
		return err1
	})

	assert.True(t, found)
	expectedKey := NewWatchTableKey(someMaxPartition, someKind, someNamespace, someName, someTs.Add(time.Hour*-5))
	assert.Equal(t, expectedKey, keyRes)
	assert.Nil(t, err)
}

func Test_getLastMatchingKeyInPartition_SameKeySearch(t *testing.T) {
	db, wt := helper_update_KubeWatchResultTable(t, (&WatchTableKey{}).SetTestKeys(), (&WatchTableKey{}).SetTestValue())
	var keyRes *WatchTableKey
	var err1 error
	var found bool
	curKey := NewWatchTableKey(someMaxPartition, someKind, someNamespace, someName, someTs)
	keyComparator := NewWatchTableKeyComparator(someKind, someNamespace, someName, someTs)
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMaxPartition, curKey, keyComparator)
		return err1
	})

	assert.False(t, found)
	assert.Equal(t, &WatchTableKey{}, keyRes)
	assert.Nil(t, err)
}

func Test_getLastMatchingKeyInPartition_NotFound(t *testing.T) {
	db, wt := helper_update_KubeWatchResultTable(t, (&WatchTableKey{}).SetTestKeys(), (&WatchTableKey{}).SetTestValue())
	var keyRes *WatchTableKey
	var err1 error
	var found bool
	curKey := NewWatchTableKey(someMaxPartition, someKind, someNamespace, someName, someTs)
	keyComparator := NewWatchTableKeyComparator(someKind+"c", someNamespace, someName, someTs)
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMinPartition, curKey, keyComparator)
		return err1
	})

	assert.False(t, found)
	assert.Equal(t, &WatchTableKey{}, keyRes)
	assert.Nil(t, err)
}

func Test_GetPreviousKey_Success(t *testing.T) {
	db, wt := helper_update_KubeWatchResultTable(t, (&WatchTableKey{}).SetTestKeys(), (&WatchTableKey{}).SetTestValue())
	var partRes *WatchTableKey
	var err1 error
	curKey := NewWatchTableKey(someMaxPartition, someKind, someNamespace, someName, someTs)
	keyComparator := NewWatchTableKeyComparator(someKind, someNamespace, someName, zeroData)
	err := db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComparator)
		return err1
	})
	assert.Nil(t, err)
	expectedKey := NewWatchTableKey(someMaxPartition, someKind, someNamespace, someName, someTs.Add(time.Hour*-5))
	assert.Equal(t, expectedKey, partRes)
}

func Test_GetPreviousKey_Fail(t *testing.T) {
	db, wt := helper_update_KubeWatchResultTable(t, (&WatchTableKey{}).SetTestKeys(), (&WatchTableKey{}).SetTestValue())
	var partRes *WatchTableKey
	var err1 error
	curKey := NewWatchTableKey(someMaxPartition, someKind, someNamespace, someName, someTs)
	keyComparator := NewWatchTableKeyComparator(someKind+"c", someNamespace, someName, zeroData)
	err := db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComparator)
		return err1
	})
	assert.NotNil(t, err)
	assert.Equal(t, &WatchTableKey{}, partRes)
}

func (*WatchTableKey) GetTestKey() string {
	k := NewWatchTableKey(someMinPartition, someKind, someNamespace, someName, someTs)
	return k.String()
}

func (*WatchTableKey) GetTestValue() *KubeWatchResult {
	return &KubeWatchResult{}
}

func (*WatchTableKey) SetTestKeys() []string {
	untyped.TestHookSetPartitionDuration(time.Hour)
	var keys []string
	var partitionId string
	for curTime := someTs; !curTime.After(someMaxTs); curTime = curTime.Add(untyped.GetPartitionDuration()) {
		// add keys in ascending order
		partitionId = untyped.GetPartitionId(curTime)
		keys = append(keys, NewWatchTableKey(partitionId, someKind, someNamespace, someName, someTs.Add(time.Hour*-5)).String())
		keys = append(keys, NewWatchTableKey(partitionId, someKind, someNamespace, someName, someTs).String())
	}

	return keys
}

func (*WatchTableKey) SetTestValue() *KubeWatchResult {
	return &KubeWatchResult{Kind: someKind}
}
