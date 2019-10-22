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

	"github.com/dgraph-io/badger"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/spf13/afero"
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

func Test_GetDirSizeRecursive(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	fs.MkdirAll(someDir, 0700)
	fs.WriteFile(somePath, []byte("abcdfdfdfd"), 0700)

	fileSize, err := getDirSizeRecursive(someDir, &fs)
	assert.Nil(t, err)
	assert.NotZero(t, fileSize)
}

func Test_cleanUpFileSizeCondition_True(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	fs.MkdirAll(someDir, 0700)
	fs.WriteFile(somePath, []byte("abcdfdfdfd"), 0700)

	flag := cleanUpFileSizeCondition(someDir, 3, &fs)
	assert.True(t, flag)
}

func Test_cleanUpFileSizeCondition_False(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	fs.MkdirAll(someDir, 0700)
	fs.WriteFile(somePath, []byte("abcdfdfdfd"), 0700)

	flag := cleanUpFileSizeCondition(someDir, 100, &fs)
	assert.False(t, flag)
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

	wtval := &typed.KubeWatchResult{Kind: someKind}
	rtval := &typed.ResourceSummary{DeletedAtEnd: false}
	ecVal := &typed.ResourceEventCounts{XXX_sizecache: int32(0)}

	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	defer db.Close()

	wt := typed.OpenKubeWatchResultTable()
	rt := typed.OpenResourceSummaryTable()
	ec := typed.OpenResourceEventCountsTable()
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

		return nil
	})
	assert.Nil(t, err)
	return db
}

func Test_doCleanup_true(t *testing.T) {
	db := help_get_db(t)
	tables := typed.NewTableList(db)

	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	fs.MkdirAll(someDir, 0700)
	fs.WriteFile(somePath, []byte("abcdfdfdfd"), 0700)

	flag, err := doCleanup(tables, someDir, time.Hour, 2, &fs)
	assert.True(t, flag)
	assert.Nil(t, err)
}

func Test_doCleanup_false(t *testing.T) {
	db := help_get_db(t)
	tables := typed.NewTableList(db)

	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	fs.MkdirAll(someDir, 0700)
	fs.WriteFile(somePath, []byte("abcdfdfdfd"), 0700)

	flag, err := doCleanup(tables, someDir, time.Hour, 1000, &fs)
	assert.False(t, flag)
	assert.Nil(t, err)
}
