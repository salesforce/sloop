/**
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
	assert.Equal(t, somePartition, k.PartitionId)
	assert.Equal(t, someNamespace, k.Namespace)
	assert.Equal(t, someName, k.Name)
}

func Test_WatchActivityKey_ValidateWorks(t *testing.T) {
	assert.Nil(t, (&WatchActivityKey{}).ValidateKey(someWatchActivityKey))
}

func helper_update_watchactivity_table(t *testing.T) (badgerwrap.DB, *WatchActivityTable) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	var keys []string
	for i := 'a'; i < 'd'; i++ {
		// add keys in ascending order
		keys = append(keys, NewWatchActivityKey(partitionId, someKind+string(i), someNamespace, someName, someUid).String())
	}
	val := &WatchActivity{}
	b, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wat := OpenWatchActivityTable()
	err = b.Update(func(txn badgerwrap.Txn) error {
		var txerr error
		for _, key := range keys {
			txerr = wat.Set(txn, key, val)
			if txerr != nil {
				return txerr
			}
		}
		// Add some keys outside the range
		txerr = txn.Set([]byte("/a/123/"), []byte{})
		if txerr != nil {
			return txerr
		}
		txerr = txn.Set([]byte("/zzz/123/"), []byte{})
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)
	return b, wat
}

func Test_WatchActivity_PutThenGet_SameData(t *testing.T) {
	db, wat := helper_update_watchactivity_table(t)

	var retval *WatchActivity
	err := db.View(func(txn badgerwrap.Txn) error {
		var txerr error
		retval, txerr = wat.Get(txn, "/watchactivity/001546398000/somekinda/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8")
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)
	assert.Nil(t, retval.ChangedAt)
}

func Test_WatchActivity_TestMinAndMaxKeys(t *testing.T) {
	db, wt := helper_update_watchactivity_table(t)
	var minKey string
	var maxKey string
	err := db.View(func(txn badgerwrap.Txn) error {
		_, minKey = wt.GetMinKey(txn)
		_, maxKey = wt.GetMaxKey(txn)
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, "/watchactivity/001546398000/somekinda/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8", minKey)
	assert.Equal(t, "/watchactivity/001546398000/somekindc/somenamespace/somename/68510937-4ffc-11e9-8e26-1418775557c8", maxKey)
}

func Test_WatchActivity_TestGetMinMaxPartitions(t *testing.T) {
	db, wt := helper_update_watchactivity_table(t)
	var minPartition string
	var maxPartition string
	var found bool
	err := db.View(func(txn badgerwrap.Txn) error {
		found, minPartition, maxPartition = wt.GetMinMaxPartitions(txn)
		return nil
	})

	assert.Nil(t, err)
	assert.True(t, found)
	assert.Equal(t, somePartition, minPartition)
	assert.Equal(t, somePartition, maxPartition)
}

func (_ *WatchActivityKey) GetTestKey() string {
	k := NewWatchActivityKey("001546398000", someKind, someNamespace, someName, someUid)
	return k.String()
}

func (_ *WatchActivityKey) GetTestValue() *WatchActivity {
	return &WatchActivity{}
}
