/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"fmt"
	"github.com/dgraph-io/badger"
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
