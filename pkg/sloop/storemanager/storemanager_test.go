/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package storemanager

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

var (
	useRealBadger = false
	testKey       = []byte("/somekey")
	testValue1    = []byte("somevalue1")
	testValue2    = []byte("somevalue2")
	someTs        = time.Date(2019, 1, 2, 3, 4, 5, 6, time.UTC)
	someDir       = "/foo"
	somePath      = "/foo/something"
	someKind      = "somekind"
	someNamespace = "somenamespace"
	someName      = "somename"
	someUid       = "123232"
)

func Test_cleanUpFileSizeCondition_True(t *testing.T) {
	stats := &storeStats{
		DiskSizeBytes: 10,
	}

	flag, ratio := cleanUpFileSizeCondition(stats, 5, 1)
	assert.True(t, flag)
	assert.Equal(t, 0.5, ratio)
}

func Test_cleanUpFileSizeCondition_False(t *testing.T) {
	stats := &storeStats{
		DiskSizeBytes: 10,
	}

	flag, ratio := cleanUpFileSizeCondition(stats, 100, 0.8)
	assert.False(t, flag)
	assert.Equal(t, 0.0, ratio)
}

func Test_cleanUpTimeCondition(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	// partition gap is smaller than time limit
	flag := cleanUpTimeCondition("001564074000", "001564077600", 3*time.Hour)
	assert.False(t, flag)

	// minPartition is illegal input
	flag = cleanUpTimeCondition("dfdfdere001564074000", "001564077600", time.Hour)
	assert.False(t, flag)

	// maxPartition is illegal input
	flag = cleanUpTimeCondition("001564074000", "dfdfdere001564077600", time.Hour)
	assert.False(t, flag)

	// partition gap is greater than time limit
	flag = cleanUpTimeCondition("001564074000", "001564077600", 20*time.Minute)
	assert.True(t, flag)
}

func help_get_db(t *testing.T) badgerwrap.DB {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	key1 := typed.NewWatchTableKey(partitionId, someKind+"a", someNamespace, someName, someTs).String()
	key2 := typed.NewResourceSummaryKey(someTs, someKind+"b", someNamespace, someName, someUid).String()
	key3 := typed.NewEventCountKey(someTs, someKind+"c", someNamespace, someName, someUid).String()
	key4 := typed.NewWatchActivityKey(untyped.GetPartitionId(someTs), someKind+"d", someNamespace, someName, someUid).String()

	wtval := &typed.KubeWatchResult{Kind: someKind}
	rtval := &typed.ResourceSummary{DeletedAtEnd: false}
	ecVal := &typed.ResourceEventCounts{XXX_sizecache: int32(0)}
	waVal := &typed.WatchActivity{XXX_sizecache: int32((0))}

	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	defer db.Close()

	wt := typed.OpenKubeWatchResultTable()
	rt := typed.OpenResourceSummaryTable()
	ec := typed.OpenResourceEventCountsTable()
	wa := typed.OpenWatchActivityTable()
	err = db.Update(func(txn badgerwrap.Txn) error {
		txerr := wt.Set(txn, key1, wtval)
		if txerr != nil {
			return txerr
		}
		txerr = rt.Set(txn, key2, rtval)
		if txerr != nil {
			return txerr
		}
		txerr = ec.Set(txn, key3, ecVal)
		if txerr != nil {
			return txerr
		}
		txerr = wa.Set(txn, key4, waVal)
		if txerr != nil {
			return txerr
		}

		return nil
	})
	assert.Nil(t, err)
	return db
}

func Test_doCleanup_true(t *testing.T) {
	db := help_get_db(t)
	tables := typed.NewTableList(db)

	stats := &storeStats{
		DiskSizeBytes: 10,
	}

	flag, _, _, err := doCleanup(tables, time.Hour, 2, stats, 10, 1)
	assert.True(t, flag)
	assert.Nil(t, err)
}

func Test_doCleanup_false(t *testing.T) {
	db := help_get_db(t)
	tables := typed.NewTableList(db)

	stats := &storeStats{
		DiskSizeBytes: 10,
	}

	flag, _, _, err := doCleanup(tables, time.Hour, 1000, stats, 10, 1)
	assert.False(t, flag)
	assert.Nil(t, err)
}

func Test_getNumberOfKeysToDelete_Success(t *testing.T) {
	db := help_get_db(t)
	keysToDelete := getNumberOfKeysToDelete(db, 0.5)
	assert.Equal(t, int64(2), keysToDelete)
}

func Test_getNumberOfKeysToDelete_Failure(t *testing.T) {
	db := help_get_db(t)
	keysToDelete := getNumberOfKeysToDelete(db, 0)
	assert.Equal(t, int64(0), keysToDelete)
}

func Test_getNumberOfKeysToDelete_TestCeiling(t *testing.T) {
	db := help_get_db(t)
	keysToDelete := getNumberOfKeysToDelete(db, 0.33)
	assert.Equal(t, int64(2), keysToDelete)
}
