package common

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/stretchr/testify/assert"
	"testing"
)

var commonPrefix = "/commonprefix/001546405200/"

func Test_Db_Utilities_DeleteKeysWithPrefix_DeleteAllKeys(t *testing.T) {
	db := helper_get_db(t)
	helper_add_keys_to_db(t, db, helper_testKeys_with_common_prefix(commonPrefix))
	err, numOfDeletedKeys, numOfKeysToDelete := DeleteKeysWithPrefix(commonPrefix, db, 10, 4)
	assert.Nil(t, err)
	assert.Equal(t, uint64(4), numOfDeletedKeys)
	assert.Equal(t, uint64(4), numOfKeysToDelete)
}

func Test_Db_Utilities_DeleteKeysWithPrefix_DeleteNoKeys(t *testing.T) {
	db := helper_get_db(t)
	helper_add_keys_to_db(t, db, helper_testKeys_with_common_prefix(commonPrefix))
	err, numOfDeletedKeys, numOfKeysToDelete := DeleteKeysWithPrefix(commonPrefix+"random", db, 10, 0)
	assert.Nil(t, err)
	assert.Equal(t, uint64(0), numOfDeletedKeys)
	assert.Equal(t, uint64(0), numOfKeysToDelete)
}

func Test_Db_Utilities_DeleteKeysWithPrefix_DeleteSomeKeys(t *testing.T) {
	db := helper_get_db(t)
	helper_add_keys_to_db(t, db, helper_testKeys_with_common_prefix(commonPrefix))
	helper_add_keys_to_db(t, db, helper_testKeys_with_common_prefix("randomStuff"+commonPrefix))
	err, numOfDeletedKeys, numOfKeysToDelete := DeleteKeysWithPrefix(commonPrefix, db, 10, 4)
	assert.Nil(t, err)
	assert.Equal(t, uint64(4), numOfDeletedKeys)
	assert.Equal(t, uint64(4), numOfKeysToDelete)
}

func helper_get_db(t *testing.T) badgerwrap.DB {
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	return db
}

func helper_add_keys_to_db(t *testing.T, db badgerwrap.DB, keys []string) badgerwrap.DB {
	err := db.Update(func(txn badgerwrap.Txn) error {
		var txerr error
		for _, key := range keys {
			txerr = txn.Set([]byte(key), []byte{})
			if txerr != nil {
				return txerr
			}
		}
		return nil
	})
	assert.Nil(t, err)
	return db
}

func helper_testKeys_with_common_prefix(prefix string) []string {
	return []string{
		// someMaxTs partition
		prefix + "Pod/user-j/sync-123/sam-partition-testdata",
		prefix + "Pod/user-j/sync-123/sam-partition-test",
		prefix + "Pod/user-t/sync-123/sam-partition-testdata",
		prefix + "Pod/user-w/sync-123/sam-partition-test",
	}
}

func Test_Db_Utilities_GetTotalKeyCount_SomeKeys(t *testing.T) {
	db := helper_get_db(t)
	helper_add_keys_to_db(t, db, helper_testKeys_with_common_prefix(commonPrefix))
	helper_add_keys_to_db(t, db, helper_testKeys_with_common_prefix("randomStuff"+commonPrefix))

	numberOfKeys := GetTotalKeyCount(db, []byte(commonPrefix))
	// expected count is 4 as each call to helper_add_keys_to_db adds keys in 4 tables, only the common prefix ones would return
	expectedNumberOfKeys := 4
	assert.Equal(t, uint64(expectedNumberOfKeys), numberOfKeys)
}

func Test_Db_Utilities_GetTotalKeyCount_NoKeys(t *testing.T) {
	db := helper_get_db(t)
	numberOfKeys := GetTotalKeyCount(db, []byte(""))
	assert.Equal(t, uint64(0), numberOfKeys)
}
