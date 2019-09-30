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
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var someTs = time.Date(2019, 1, 2, 3, 4, 5, 6, time.UTC)

const someKind = "somekind"
const someNamespace = "somenamespace"
const someName = "somename"
const somePartition = "001546398000"

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
	assert.Equal(t, somePartition, k.PartitionId)
	assert.Equal(t, someNamespace, k.Namespace)
	assert.Equal(t, someName, k.Name)
	assert.Equal(t, someTs, k.Timestamp)
}

func Test_WatchTableKey_ValidateWorks(t *testing.T) {
	testKey := "/watch/001562961600/ReplicaSet/mesh-control-plane/istio-pilot-56f7d9848/1562963507608345756"
	assert.Nil(t, (&WatchTableKey{}).ValidateKey(testKey))
}

func helper_update_watch_table(t *testing.T) (badgerwrap.DB, *KubeWatchResultTable) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	var keys []string
	for i := 'a'; i < 'd'; i++ {
		// add keys in ascending order
		keys = append(keys, NewWatchTableKey(partitionId, someKind+string(i), someNamespace, someName, someTs).String())
	}
	val := &KubeWatchResult{Kind: someKind}
	b, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wt := OpenKubeWatchResultTable()
	err = b.Update(func(txn badgerwrap.Txn) error {
		var txerr error
		for _, key := range keys {
			txerr = wt.Set(txn, key, val)
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
	return b, wt
}

func Test_WatchTable_PutThenGet_SameData(t *testing.T) {
	db, wt := helper_update_watch_table(t)

	var retval *KubeWatchResult
	err := db.View(func(txn badgerwrap.Txn) error {
		var txerr error
		retval, txerr = wt.Get(txn, "/watch/001546398000/somekinda/somenamespace/somename/1546398245000000006")
		if txerr != nil {
			return txerr
		}
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, someKind, retval.Kind)
}

func Test_WatchTable_TestMinAndMaxKeys(t *testing.T) {
	db, wt := helper_update_watch_table(t)
	var minKey string
	var maxKey string
	err := db.View(func(txn badgerwrap.Txn) error {
		_, minKey = wt.GetMinKey(txn)
		_, maxKey = wt.GetMaxKey(txn)
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, "/watch/001546398000/somekinda/somenamespace/somename/1546398245000000006", minKey)
	assert.Equal(t, "/watch/001546398000/somekindc/somenamespace/somename/1546398245000000006", maxKey)
}

func Test_WatchTable_TestGetMinMaxPartitions(t *testing.T) {
	db, wt := helper_update_watch_table(t)
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

func (_ *WatchTableKey) GetTestKey() string {
	k := NewWatchTableKey("001546398000", "someKind", "someNamespace", "someName", someTs)
	return k.String()
}

func (_ *WatchTableKey) GetTestValue() *KubeWatchResult {
	return &KubeWatchResult{}
}
