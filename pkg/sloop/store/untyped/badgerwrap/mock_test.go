/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package badgerwrap

import (
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

// Turn then on when writing new tests, but leave off when you check in
var useRealBadger = false

func helper_OpenDb(t *testing.T) DB {
	if useRealBadger {
		// Badger Data DB
		dataDir, err := ioutil.TempDir("", "data")
		assert.Nil(t, err)
		options := badger.DefaultOptions(dataDir)
		db, err := (&BadgerFactory{}).Open(options)
		assert.Nil(t, err)
		return db
	} else {
		options := badger.DefaultOptions("")
		db, err := (&MockFactory{}).Open(options)
		assert.Nil(t, err)
		return db
	}
}

func helper_Set(t *testing.T, db DB, key []byte, value []byte) {
	err := db.Update(
		func(t Txn) error {
			t.Set(key, value)
			return nil
		})
	assert.Nil(t, err)
}

// There are 3 ways to get the value of an item.  We are ensuring
// they all match.  If this slows down tests we can make it optional
func helper_Get(t *testing.T, db DB, key []byte) ([]byte, error) {
	var actualValueFn []byte
	err := db.View(
		func(tx Txn) error {
			item, err2 := tx.Get(key)
			if err2 != nil {
				return err2
			}
			assert.Equal(t, key, item.Key())

			// Grab value first with a function
			item.Value(
				func(val []byte) error {
					actualValueFn = val
					return nil
				})

			// Get it a second time as a copy and make sure they match
			var actualValueCopy []byte
			actualValueCopy, err2 = item.ValueCopy([]byte{})
			assert.Nil(t, err2)
			assert.Equal(t, actualValueFn, actualValueCopy)

			// Get it a third time with writing to an existing slice
			var actualValueCopyExisintSlice = make([]byte, len(actualValueFn))
			_, err2 = item.ValueCopy(actualValueCopyExisintSlice)
			assert.Nil(t, err2)
			assert.Equal(t, actualValueFn, actualValueCopyExisintSlice)

			return nil
		})
	return actualValueFn, err
}

func helper_GetNoError(t *testing.T, db DB, key []byte) []byte {
	data, err := helper_Get(t, db, key)
	assert.Nil(t, err)
	return data
}

func helper_iterateKeys(db DB, opt badger.IteratorOptions) []string {
	actual := []string{}
	db.View(func(txn Txn) error {
		i := txn.NewIterator(opt)
		defer i.Close()
		for i.Rewind(); i.Valid(); i.Next() {
			actual = append(actual, string(i.Item().Key()))
		}
		return nil
	})
	return actual
}

func helper_iterateKeysPrefix(db DB, opt badger.IteratorOptions, seek string, prefix string) []string {
	actual := []string{}
	db.View(func(txn Txn) error {
		i := txn.NewIterator(opt)
		defer i.Close()

		// Split out for easier debugging
		// for i.Seek([]byte(prefix)); i.ValidForPrefix([]byte(prefix)); i.Next() {
		// }
		i.Seek([]byte(seek))
		for {
			if !i.ValidForPrefix([]byte(prefix)) {
				break
			}
			actual = append(actual, string(i.Item().Key()))
			i.Next()
		}
		return nil
	})
	return actual
}

var testKey = []byte("/somekey")
var testValue1 = []byte("somevalue1")
var testValue2 = []byte("somevalue2")

func Test_MockBadger_GetMissingKey_ReturnsCorrectError(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()
	_, err := helper_Get(t, db, testKey)
	assert.Equal(t, "Key not found", fmt.Sprintf("%v", err.Error()))
}

func Test_MockBadger_PutAndGet_ValuesMatch(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()
	helper_Set(t, db, testKey, testValue1)
	actualValue := helper_GetNoError(t, db, testKey)
	assert.Equal(t, testValue1, actualValue)
}

func Test_MockBadger_WriteTwoValuesToSameKey_LatestIsReturned(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()
	helper_Set(t, db, testKey, testValue1)
	helper_Set(t, db, testKey, testValue2)
	actualValue := helper_GetNoError(t, db, testKey)
	assert.Equal(t, testValue2, actualValue)
}

func Test_MockBadger_AddThenDelete_NotFound(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()

	helper_Set(t, db, testKey, testValue1)

	err := db.Update(
		func(txn Txn) error {
			txn.Delete(testKey)
			return nil
		})
	assert.Nil(t, err)

	_, err = helper_Get(t, db, testKey)
	assert.Equal(t, "Key not found", fmt.Sprintf("%v", err.Error()))
}

func Test_MockBadger_DeleteAMissingKey_NoError(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()

	err := db.Update(
		func(txn Txn) error {
			err2 := txn.Delete(testKey)
			assert.Nil(t, err2)
			return nil
		})
	assert.Nil(t, err)
}

func Test_MockBadger_SetAnEmptyKey_NoError(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()

	helper_Set(t, db, []byte{}, testValue1)
}

func Test_MockBadger_IterateAllKeys(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()

	expected := []string{"/a1", "/a2", "/a3", "/a4"}
	// Include some dupes
	for _, key := range []string{"/a4", "/a1", "/a3", "/a2", "/a4"} {
		helper_Set(t, db, []byte(key), []byte{})
	}

	actual := helper_iterateKeys(db, badger.DefaultIteratorOptions)
	assert.Equal(t, expected, actual)
}

func Test_MockBadger_IterateAllKeysBackwards(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()

	expected := []string{"/a4", "/a3", "/a2", "/a1"}
	// Include some dupes
	for _, key := range []string{"/a4", "/a1", "/a3", "/a2", "/a4"} {
		helper_Set(t, db, []byte(key), []byte{})
	}

	opt := badger.DefaultIteratorOptions
	opt.Reverse = true
	actual := helper_iterateKeys(db, opt)
	assert.Equal(t, expected, actual)
}

func Test_MockBadger_IterateAllKeysWithPrefix(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()

	expected := []string{"/b/1", "/b/4"}
	// Include some dupes
	for _, key := range []string{"/a/1", "/a/2", "/b/1", "/b/4", "/c/1", "/c/2"} {
		helper_Set(t, db, []byte(key), []byte{})
	}

	actual := helper_iterateKeysPrefix(db, badger.DefaultIteratorOptions, "/b/", "/b/")
	assert.Equal(t, expected, actual)
}

// This feels like a bug in badger.  We need to seek to the key after our prefix
// Sounds like its by design: https://github.com/dgraph-io/badger/issues/436
// "/b0" is one past "/b/"
func Test_MockBadger_IterateAllKeysWithPrefixBackwards(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()

	expected := []string{"/b/4", "/b/1"}
	// Include some dupes
	for _, key := range []string{"/a/1", "/a/2", "/b/1", "/b/4", "/c/1", "/c/2"} {
		helper_Set(t, db, []byte(key), []byte{})
	}

	opt := badger.DefaultIteratorOptions
	opt.Reverse = true
	actual := helper_iterateKeysPrefix(db, opt, "/b0", "/b/")
	assert.Equal(t, expected, actual)
}

func Test_MockBadger_DropPrefix_OK(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()

	expected := []string{"/b/1", "/b/4"}

	for _, key := range []string{"/a/1", "/a/2", "/b/1", "/b/4", "/c/1", "/c/2"} {
		helper_Set(t, db, []byte(key), []byte{})
	}

	actual := helper_iterateKeysPrefix(db, badger.DefaultIteratorOptions, "/b/", "/b/")
	assert.Equal(t, expected, actual)

	// start drop prefix with /b
	db.DropPrefix([]byte("/b"))
	actual = helper_iterateKeysPrefix(db, badger.DefaultIteratorOptions, "/b/", "/b/")
	assert.Len(t, actual, 0)
}

func Test_MockBadger_DropPrefix_Fail(t *testing.T) {
	db := helper_OpenDb(t)
	defer db.Close()
	for _, key := range []string{"/a/1", "/a/2", "/b/1", "/b/4", "/c/1", "/c/2"} {
		helper_Set(t, db, []byte(key), testValue1)
	}

	db.DropPrefix([]byte("/x"))

	for _, key := range []string{"/a/1", "/a/2", "/b/1", "/b/4", "/c/1", "/c/2"} {
		data, err := helper_Get(t, db, []byte(key))
		assert.Nil(t, err)
		assert.Equal(t, len(testValue1), len(data))
	}
}
