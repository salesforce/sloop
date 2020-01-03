/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/salesforce/sloop/pkg/sloop/test/assertex"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const someUid = "68510937-4ffc-11e9-8e26-1418775557c8"

var someFirstSeenTime = time.Date(2019, 3, 4, 3, 4, 5, 6, time.UTC)

func Test_ResourceSummaryTableKey_OutputCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	k := NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid)
	assert.Equal(t, "/ressum/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8", k.String())
}

func Test_ResourceSummaryTableKey_ParseCorrect(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	k := &ResourceSummaryKey{}
	err := k.Parse("/ressum/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8")
	assert.Nil(t, err)
	assert.Equal(t, "001546398000", k.PartitionId)
	assert.Equal(t, someNamespace, k.Namespace)
	assert.Equal(t, someName, k.Name)
	assert.Equal(t, someUid, k.Uid)
}

func Test_ResourceSummary_PutThenGet_SameData(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	createTimeProto, err := ptypes.TimestampProto(someFirstSeenTime)
	assert.Nil(t, err)

	key := NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid).String()
	val := &ResourceSummary{FirstSeen: createTimeProto}

	b, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wt := OpenResourceSummaryTable()

	err = b.Update(func(txn badgerwrap.Txn) error {
		txerr := wt.Set(txn, key, val)
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)

	var retval *ResourceSummary
	err = b.View(func(txn badgerwrap.Txn) error {
		var txerr error
		retval, txerr = wt.Get(txn, key)
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)

	assertex.ProtoEqual(t, val.FirstSeen, retval.FirstSeen)
}

func Test_ResourceSummary_RangeRead(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	createTimeProto, err := ptypes.TimestampProto(someFirstSeenTime)
	assert.Nil(t, err)

	key1 := NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid+"a")
	key2 := NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid+"b")
	val := &ResourceSummary{FirstSeen: createTimeProto}

	b, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wt := OpenResourceSummaryTable()

	err = b.Update(func(txn badgerwrap.Txn) error {
		txerr := wt.Set(txn, key1.String(), val)
		if txerr != nil {
			return txerr
		}
		txerr = wt.Set(txn, key2.String(), val)
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)

	var retval map[ResourceSummaryKey]*ResourceSummary
	err = b.View(func(txn badgerwrap.Txn) error {
		var txerr error
		retval, _, txerr = wt.RangeRead(txn, nil, func(k string) bool { return true }, func(r *ResourceSummary) bool { return true }, someTs, someTs)
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)

	assert.Contains(t, retval, *key1)
	assert.Contains(t, retval, *key2)
	assertex.ProtoEqual(t, val, retval[*key1])
	assertex.ProtoEqual(t, val, retval[*key2])
}

func Test_ResourceSummary_RangeReadWithKeyPredicate(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	createTimeProto, err := ptypes.TimestampProto(someFirstSeenTime)
	assert.Nil(t, err)

	key1 := NewResourceSummaryKey(someTs, someKind, someNamespace, someName+"a", someUid)
	key2 := NewResourceSummaryKey(someTs, someKind, someNamespace+"b", someName+"b", someUid)
	val := &ResourceSummary{FirstSeen: createTimeProto}

	b, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wt := OpenResourceSummaryTable()

	err = b.Update(func(txn badgerwrap.Txn) error {
		txerr := wt.Set(txn, key1.String(), val)
		if txerr != nil {
			return txerr
		}
		txerr = wt.Set(txn, key2.String(), val)
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)

	var retval map[ResourceSummaryKey]*ResourceSummary
	err = b.View(func(txn badgerwrap.Txn) error {
		var txerr error
		retval, _, txerr = wt.RangeRead(txn, nil, func(k string) bool {
			key := &ResourceSummaryKey{}
			err2 := key.Parse(k)
			assert.Nil(t, err2)
			return key.Namespace == someNamespace+"b"
		}, func(r *ResourceSummary) bool { return true }, someTs, someTs)
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)

	assert.Len(t, retval, 1)
	assert.Contains(t, retval, *key2)
	assert.NotContains(t, retval, *key1)
	assertex.ProtoEqual(t, val, retval[*key2])
}

func helper_update_resourcesummary_table(t *testing.T, keysFn func() []string) (badgerwrap.DB, *ResourceSummaryTable) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	createTimeProto, err := ptypes.TimestampProto(someFirstSeenTime)
	assert.Nil(t, err)
	keys := keysFn()
	val := &ResourceSummary{FirstSeen: createTimeProto}
	b, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	rt := OpenResourceSummaryTable()
	err = b.Update(func(txn badgerwrap.Txn) error {
		var txerr error
		for _, key := range keys {
			txerr = rt.Set(txn, key, val)
			if txerr != nil {
				return txerr
			}
		}
		return nil
	})
	assert.Nil(t, err)
	return b, rt
}

func Test_ResourceSummary_TestMinAndMaxKeys(t *testing.T) {
	keysFn := func() []string {
		var keys []string
		for i := 'a'; i < 'd'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid+string(i)).String())
		}
		return keys
	}
	db, rt := helper_update_resourcesummary_table(t, keysFn)
	var minKey string
	var maxKey string
	err := db.View(func(txn badgerwrap.Txn) error {
		_, minKey = rt.GetMinKey(txn)
		_, maxKey = rt.GetMaxKey(txn)
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, "/ressum/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8a", minKey)
	assert.Equal(t, "/ressum/001546398000/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8c", maxKey)
}

func Test_ResourceSummary_TestGetMinMaxParititons(t *testing.T) {
	keysFn := func() []string {
		var keys []string
		for i := 'a'; i < 'd'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid+string(i)).String())
		}
		return keys
	}
	db, rt := helper_update_resourcesummary_table(t, keysFn)
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

func Test_ResourceSummary_RangeReadWithTimeRange(t *testing.T) {
	keysFn := func() []string {
		var keys []string
		for i := 'a'; i < 'c'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid+string(i)).String())
		}
		for i := 'c'; i < 'e'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someMiddleTs, someKind, someNamespace, someName, someUid+string(i)).String())
		}
		for i := 'e'; i < 'g'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someMaxTs, someKind, someNamespace, someName, someUid+string(i)).String())
		}
		return keys
	}
	db, rst := helper_update_resourcesummary_table(t, keysFn)
	var retval map[ResourceSummaryKey]*ResourceSummary
	err := db.View(func(txn badgerwrap.Txn) error {
		var txerr error
		// someTs starts with 4 minutes, subtract 5 minutes to not include partitions above (someTs + 2hours)
		retval, _, txerr = rst.RangeRead(txn, nil, func(k string) bool { return true }, func(r *ResourceSummary) bool { return true }, someTs.Add(1*time.Hour), someTs.Add(2*time.Hour-5*time.Minute))
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)
	assert.Len(t, retval, 2)
	expectedKey := &ResourceSummaryKey{}
	err = expectedKey.Parse("/ressum/001546401600/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8c")
	assert.Nil(t, err)
	assert.Contains(t, retval, *expectedKey)
	err = expectedKey.Parse("/ressum/001546401600/somekind/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8d")
	assert.Nil(t, err)
	assert.Contains(t, retval, *expectedKey)
}

func Test_ResourceSummary_getLastMatchingKeyInPartition_FoundInPreviousPartition(t *testing.T) {
	db, wt := helper_update_ResourceSummaryTable(t, (&ResourceSummaryKey{}).SetTestKeys(), (&ResourceSummaryKey{}).SetTestValue())
	var keyRes *ResourceSummaryKey
	var err1 error
	var found bool
	curKey := NewResourceSummaryKey(someMaxTs, someKind, someNamespace, someName, someUid+"c")
	keyComparator := NewResourceSummaryKeyComparator(someKind, someNamespace, someName, someUid+"b")
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMiddlePartition, curKey, keyComparator)
		return err1
	})
	assert.True(t, found)
	expectedKey := NewResourceSummaryKey(someMiddleTs, someKind, someNamespace, someName, someUid+"b")
	assert.Equal(t, expectedKey, keyRes)
	assert.Nil(t, err)
}

func Test_ResourceSummary_getLastMatchingKeyInPartition_FoundInSamePartition(t *testing.T) {
	db, wt := helper_update_ResourceSummaryTable(t, (&ResourceSummaryKey{}).SetTestKeys(), (&ResourceSummaryKey{}).SetTestValue())
	var keyRes *ResourceSummaryKey
	var err1 error
	var found bool
	curKey := NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid+"a")
	keyComparator := NewResourceSummaryKeyComparator(someKind, someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMinPartition, curKey, keyComparator)
		return err1
	})

	assert.True(t, found)
	expectedKey := NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid)
	assert.Equal(t, expectedKey, keyRes)
	assert.Nil(t, err)
}

func Test_ResourceSummary_getLastMatchingKeyInPartition_SameKeySearch(t *testing.T) {
	db, wt := helper_update_ResourceSummaryTable(t, (&ResourceSummaryKey{}).SetTestKeys(), (&ResourceSummaryKey{}).SetTestValue())
	var keyRes *ResourceSummaryKey
	var err1 error
	var found bool
	curKey := NewResourceSummaryKey(someTs, someKind, someNamespace, someName, someUid+"a")
	keyComparator := NewResourceSummaryKeyComparator(someKind, someNamespace, someName, someUid+"a")
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMinPartition, curKey, keyComparator)
		return err1
	})

	assert.False(t, found)
	assert.Equal(t, &ResourceSummaryKey{}, keyRes)
	assert.Nil(t, err)
}

func Test_ResourceSummary_getLastMatchingKeyInPartition_NotFound(t *testing.T) {
	db, wt := helper_update_ResourceSummaryTable(t, (&ResourceSummaryKey{}).SetTestKeys(), (&ResourceSummaryKey{}).SetTestValue())
	var keyRes *ResourceSummaryKey
	var err1 error
	var found bool
	curKey := NewResourceSummaryKey(someMaxTs, someKind, someNamespace, someName, someUid)
	keyComparator := NewResourceSummaryKeyComparator(someKind+"c", someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		found, keyRes, err1 = wt.getLastMatchingKeyInPartition(txn, someMinPartition, curKey, keyComparator)
		return err1
	})

	assert.False(t, found)
	assert.Equal(t, &ResourceSummaryKey{}, keyRes)
	assert.Nil(t, err)
}

func Test_ResourceSummary_GetPreviousKey_Success(t *testing.T) {
	db, wt := helper_update_ResourceSummaryTable(t, (&ResourceSummaryKey{}).SetTestKeys(), (&ResourceSummaryKey{}).SetTestValue())
	var partRes *ResourceSummaryKey
	var err1 error
	curKey := NewResourceSummaryKey(someMaxTs, someKind, someNamespace, someName, someUid+"c")
	keyComparator := NewResourceSummaryKeyComparator(someKind, someNamespace, someName, someUid+"b")
	err := db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComparator)
		return err1
	})
	assert.Nil(t, err)
	expectedKey := NewResourceSummaryKey(someTs.Add(1*time.Hour), someKind, someNamespace, someName, someUid+"b")
	assert.Equal(t, expectedKey, partRes)
}

func Test_ResourceSummary_GetPreviousKey_Fail(t *testing.T) {
	db, wt := helper_update_ResourceSummaryTable(t, (&ResourceSummaryKey{}).SetTestKeys(), (&ResourceSummaryKey{}).SetTestValue())
	var partRes *ResourceSummaryKey
	var err1 error
	curKey := NewResourceSummaryKey(someTs.Add(2*time.Hour), someKind, someNamespace, someName, someUid)
	keyComparator := NewResourceSummaryKeyComparator(someKind+"b", someNamespace, someName, someUid)
	err := db.View(func(txn badgerwrap.Txn) error {
		partRes, err1 = wt.GetPreviousKey(txn, curKey, keyComparator)
		return err1
	})
	assert.NotNil(t, err)
	assert.Equal(t, &ResourceSummaryKey{}, partRes)
}

func (*ResourceSummaryKey) GetTestKey() string {
	k := NewResourceSummaryKey(someTs, "someKind", "someNamespace", "someName", "someUuid")
	return k.String()
}

func (*ResourceSummaryKey) GetTestValue() *ResourceSummary {
	return &ResourceSummary{}
}

func (*ResourceSummaryKey) SetTestKeys() []string {
	untyped.TestHookSetPartitionDuration(time.Hour)
	var keys []string
	i := 'a'
	for curTime := someTs; !curTime.After(someMaxTs); curTime = curTime.Add(untyped.GetPartitionDuration()) {
		// add keys in ascending order
		keys = append(keys, NewResourceSummaryKey(curTime, someKind, someNamespace, someName, someUid).String())
		keys = append(keys, NewResourceSummaryKey(curTime, someKind, someNamespace, someName, someUid+string(i)).String())
		i++
	}

	return keys
}

func (*ResourceSummaryKey) SetTestValue() *ResourceSummary {
	createTimeProto, _ := ptypes.TimestampProto(someFirstSeenTime)
	return &ResourceSummary{FirstSeen: createTimeProto}
}
