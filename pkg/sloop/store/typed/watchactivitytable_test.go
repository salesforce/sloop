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

const (
	someWatchActivityKey = "/watchactivity/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8"
)

func Test_WatchActivityKey_OutputCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	k := NewWatchActivityKey(partitionId, someKind, someNamespace, someName, someUid)
	assert.Equal(t, someWatchActivityKey, k.String())
}

func Test_WatchActivityKey_ParseCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	k := &WatchActivityKey{}
	err := k.Parse(someWatchActivityKey)
	assert.Nil(t, err)
	assert.Equal(t, someMinPartition, k.PartitionId)
	assert.Equal(t, someNamespace, k.Namespace)
	assert.Equal(t, someName, k.Name)
}

func Test_WatchActivityKey_ValidateWorks(t *testing.T) {
	assert.Nil(t, (&WatchActivityKey{}).ValidateKey(someWatchActivityKey))
}

func Test_WatchActivity_PutThenGet_SameData(t *testing.T) {
	db, wat := helper_update_WatchActivityTable(t, (&WatchActivityKey{}).SetTestKeys(), (&WatchActivityKey{}).SetTestValue())
	var retval *WatchActivity
	err := db.View(func(txn badgerwrap.Txn) error {
		var txerr error
		retval, txerr = wat.Get(txn, "/watchactivity/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8")
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)
	assert.Nil(t, retval.ChangedAt)
}

func Test_WatchActivity_TestMinAndMaxKeys(t *testing.T) {
	db, wt := helper_update_WatchActivityTable(t, (&WatchActivityKey{}).SetTestKeys(), (&WatchActivityKey{}).SetTestValue())
	var minKey string
	var maxKey string
	err := db.View(func(txn badgerwrap.Txn) error {
		_, minKey = wt.GetMinKey(txn)
		_, maxKey = wt.GetMaxKey(txn)
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, "/watchactivity/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8", minKey)
	assert.Equal(t, "/watchactivity/001546405200/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8c", maxKey)
}

func Test_WatchActivity_TestGetMinMaxPartitions(t *testing.T) {
	db, wt := helper_update_WatchActivityTable(t, (&WatchActivityKey{}).SetTestKeys(), (&WatchActivityKey{}).SetTestValue())
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

func Test_WatchActivity_getLastMatchingKeyInPartition_FoundInPreviousPartition(t *testing.T) {
	db, wt := helper_update_WatchActivityTable(t, (&WatchActivityKey{}).SetTestKeys(), (&WatchActivityKey{}).GetTestValue())
	var keyRes *WatchActivityKey
	var err1 error
	var found bool

	curKey := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid)
	keyPrefix := NewWatchActivityKey(someMiddlePartition, someKind, someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMiddlePartition, curKey, keyPrefix)
		return err1
	})
	assert.True(t, found)
	expectedKey := NewWatchActivityKey(someMiddlePartition, someKind, someNamespace, someName, someUid+"b")
	assert.Equal(t, expectedKey, keyRes)
	assert.Nil(t, err)
}

func Test_WatchActivity_getLastMatchingKeyInPartition_FoundInSamePartition(t *testing.T) {
	db, wt := helper_update_WatchActivityTable(t, (&WatchActivityKey{}).SetTestKeys(), (&WatchActivityKey{}).GetTestValue())
	var keyRes *WatchActivityKey
	var err1 error
	var found bool
	curKey := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid+"a")
	keyPrefix := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMaxPartition, curKey, keyPrefix)
		return err1
	})

	assert.True(t, found)
	expectedKey := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid)
	assert.Equal(t, expectedKey, keyRes)
	assert.Nil(t, err)
}

func Test_WatchActivity_getLastMatchingKeyInPartition_SameKeySearch_NotFound(t *testing.T) {
	db, wt := helper_update_WatchActivityTable(t, (&WatchActivityKey{}).SetTestKeys(), (&WatchActivityKey{}).GetTestValue())
	var keyRes *WatchActivityKey
	var err1 error
	var found bool
	curKey := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid+"c")
	keyPrefix := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid+"c")
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMaxPartition, curKey, keyPrefix)
		return err1
	})

	assert.False(t, found)
	assert.Equal(t, &WatchActivityKey{}, keyRes)
	assert.Nil(t, err)
}

func Test_WatchActivity_getLastMatchingKeyInPartition_NotFound(t *testing.T) {
	db, wt := helper_update_WatchActivityTable(t, (&WatchActivityKey{}).SetTestKeys(), (&WatchActivityKey{}).GetTestValue())
	var keyRes *WatchActivityKey
	var err1 error
	var found bool
	curKey := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid)
	keyPrefix := NewWatchActivityKey(someMinPartition, someKind+"c", someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMinPartition, curKey, keyPrefix)
		return err1
	})

	assert.False(t, found)
	assert.Equal(t, &WatchActivityKey{}, keyRes)
	assert.Nil(t, err)
}

func Test_WatchActivity_GetPreviousKey_Success(t *testing.T) {
	db, wt := helper_update_WatchActivityTable(t, (&WatchActivityKey{}).SetTestKeys(), (&WatchActivityKey{}).GetTestValue())
	var partRes *WatchActivityKey
	var err1 error
	curKey := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid+"c")
	keyComarator := NewWatchActivityKeyComparator(someKind, someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComarator)
		return err1
	})
	assert.Nil(t, err)
	expectedKey := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid)
	assert.Equal(t, expectedKey, partRes)
}

func Test_WatchActivity_GetPreviousKey_Fail(t *testing.T) {
	db, wt := helper_update_WatchActivityTable(t, (&WatchActivityKey{}).SetTestKeys(), (&WatchActivityKey{}).GetTestValue())
	var partRes *WatchActivityKey
	var err1 error
	curKey := NewWatchActivityKey(someMaxPartition, someKind, someNamespace, someName, someUid)
	keyComarator := NewWatchActivityKeyComparator(someKind+"a", someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComarator)
		return err1
	})
	assert.NotNil(t, err)
	assert.Equal(t, &WatchActivityKey{}, partRes)
}

func (*WatchActivityKey) GetTestKey() string {
	k := NewWatchActivityKey(someMinPartition, someKind, someNamespace, someName, someUid)
	return k.String()
}

func (*WatchActivityKey) GetTestValue() *WatchActivity {
	return &WatchActivity{}
}

func (*WatchActivityKey) SetTestKeys() []string {
	untyped.TestHookSetPartitionDuration(time.Hour)
	var keys []string
	var partitionId string
	gap := 0
	for i := 'a'; i < 'd'; i++ {
		// add keys in ascending order
		partitionId = untyped.GetPartitionId(someTs.Add(time.Hour * time.Duration(gap)))
		keys = append(keys, NewWatchActivityKey(partitionId, someKind, someNamespace, someName, someUid).String())
		keys = append(keys, NewWatchActivityKey(partitionId, someKind, someNamespace, someName, someUid+string(i)).String())
		gap++
	}
	return keys
}

func (*WatchActivityKey) SetTestValue() *WatchActivity {
	return &WatchActivity{}
}
