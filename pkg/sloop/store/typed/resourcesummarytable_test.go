/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"github.com/dgraph-io/badger"
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

	key1 := NewResourceSummaryKey(someTs, someKind, someNamespace, someName+"a", someUid)
	key2 := NewResourceSummaryKey(someTs, someKind, someNamespace, someName+"b", someUid)
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
		retval, _, txerr = wt.RangeRead(txn, func(k string) bool { return true }, func(r *ResourceSummary) bool { return true }, someTs, someTs)
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
		retval, _, txerr = wt.RangeRead(txn, func(k string) bool {
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
	return b, rt
}

func Test_ResourceSummary_TestMinAndMaxKeys(t *testing.T) {
	keysFn := func() []string {
		var keys []string
		for i := 'a'; i < 'd'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someTs, someKind, someNamespace, someName+string(i), someUid).String())
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
	assert.Equal(t, "/ressum/001546398000/somekind/somenamespace/somenamea/68510937-4ffc-11e9-8e26-1418775557c8", minKey)
	assert.Equal(t, "/ressum/001546398000/somekind/somenamespace/somenamec/68510937-4ffc-11e9-8e26-1418775557c8", maxKey)
}

func Test_ResourceSummary_TestGetMinMaxParititons(t *testing.T) {
	keysFn := func() []string {
		var keys []string
		for i := 'a'; i < 'd'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someTs, someKind, someNamespace, someName+string(i), someUid).String())
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
	var someTs = time.Date(2019, 1, 2, 3, 4, 5, 6, time.UTC)
	keysFn := func() []string {
		var keys []string
		for i := 'a'; i < 'c'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someTs, someKind, someNamespace, someName+string(i), someUid).String())
		}
		for i := 'c'; i < 'e'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someTs.Add(1*time.Hour), someKind, someNamespace, someName+string(i), someUid).String())
		}
		for i := 'e'; i < 'g'; i++ {
			// add keys in ascending order
			keys = append(keys, NewResourceSummaryKey(someTs.Add(2*time.Hour), someKind, someNamespace, someName+string(i), someUid).String())
		}
		return keys
	}
	db, rst := helper_update_resourcesummary_table(t, keysFn)
	var retval map[ResourceSummaryKey]*ResourceSummary
	err := db.View(func(txn badgerwrap.Txn) error {
		var txerr error
		// someTs starts with 4 minutes, subtract 5 minutes to not include partitions above (someTs + 2hours)
		retval, _, txerr = rst.RangeRead(txn, func(k string) bool { return true }, func(r *ResourceSummary) bool { return true }, someTs.Add(1*time.Hour), someTs.Add(2*time.Hour-5*time.Minute))
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)
	assert.Len(t, retval, 2)
	expectedKey := &ResourceSummaryKey{}
	err = expectedKey.Parse("/ressum/001546401600/somekind/somenamespace/somenamec/68510937-4ffc-11e9-8e26-1418775557c8")
	assert.Nil(t, err)
	assert.Contains(t, retval, *expectedKey)
	err = expectedKey.Parse("/ressum/001546401600/somekind/somenamespace/somenamed/68510937-4ffc-11e9-8e26-1418775557c8")
	assert.Nil(t, err)
	assert.Contains(t, retval, *expectedKey)
}

func (_ *ResourceSummaryKey) GetTestKey() string {
	k := NewResourceSummaryKey(someTs, "someKind", "someNamespace", "someName", "someUuid")
	return k.String()
}

func (_ *ResourceSummaryKey) GetTestValue() *ResourceSummary {
	return &ResourceSummary{}
}
