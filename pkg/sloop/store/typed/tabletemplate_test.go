/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"fmt"
	badger "github.com/dgraph-io/badger/v2"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
	"time"
)

//go:generate genny -in=$GOFILE -out=watchtablegen_test.go gen "ValueType=KubeWatchResult KeyType=WatchTableKey"
//go:generate genny -in=$GOFILE -out=resourcesummarytablegen_test.go gen "ValueType=ResourceSummary KeyType=ResourceSummaryKey"
//go:generate genny -in=$GOFILE -out=eventcounttablegen_test.go gen "ValueType=ResourceEventCounts KeyType=EventCountKey"
//go:generate genny -in=$GOFILE -out=watchactivitytablegen_test.go gen "ValueType=WatchActivity KeyType=WatchActivityKey"

func helper_ValueType_ShouldSkip() bool {
	// Tests will not work on the fake types in the template, but we want to run tests on real objects
	if "typed.Value"+"Type" == fmt.Sprint(reflect.TypeOf(ValueType{})) {
		fmt.Printf("Skipping unit test")
		return true
	}
	return false
}

func Test_ValueTypeTable_SetWorks(t *testing.T) {
	if helper_ValueType_ShouldSkip() {
		return
	}

	untyped.TestHookSetPartitionDuration(time.Hour * 24)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	err = db.Update(func(txn badgerwrap.Txn) error {
		k := (&KeyType{}).GetTestKey()
		vt := OpenValueTypeTable()
		err2 := vt.Set(txn, k, (&KeyType{}).GetTestValue())
		assert.Nil(t, err2)
		return nil
	})
	assert.Nil(t, err)
}

func helper_update_ValueTypeTable(t *testing.T, keys []string, val *ValueType) (badgerwrap.DB, *ValueTypeTable) {
	b, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wt := OpenValueTypeTable()
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

func Test_ValueTypeTable_GetUniquePartitionList_Success(t *testing.T) {
	if helper_ValueType_ShouldSkip() {
		return
	}

	db, wt := helper_update_ValueTypeTable(t, (&KeyType{}).SetTestKeys(), (&KeyType{}).SetTestValue())
	var partList []string
	var err1 error
	err := db.View(func(txn badgerwrap.Txn) error {
		partList, err1 = wt.GetUniquePartitionList(txn)
		return nil
	})
	assert.Nil(t, err)
	assert.Nil(t, err1)
	assert.Len(t, partList, 3)
	assert.Contains(t, partList, someMinPartition)
	assert.Contains(t, partList, someMiddlePartition)
	assert.Contains(t, partList, someMaxPartition)
}

func Test_ValueTypeTable_GetUniquePartitionList_EmptyPartition(t *testing.T) {
	if helper_ValueType_ShouldSkip() {
		return
	}

	db, wt := helper_update_ValueTypeTable(t, []string{}, &ValueType{})
	var partList []string
	var err1 error
	err := db.View(func(txn badgerwrap.Txn) error {
		partList, err1 = wt.GetUniquePartitionList(txn)
		return err1
	})
	assert.Nil(t, err)
	assert.Len(t, partList, 0)
}
