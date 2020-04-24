package common

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

func deleteKeys(db badgerwrap.DB, keysForDelete [][]byte) (error, uint64) {
	var deletedKeysInThisBatch uint64 = 0
	err := db.Update(func(txn badgerwrap.Txn) error {
		for _, key := range keysForDelete {
			err := txn.Delete(key)
			if err != nil {
				return err
			}
			deletedKeysInThisBatch++
		}
		return nil
	})

	if err != nil {
		return err, deletedKeysInThisBatch
	}

	return nil, deletedKeysInThisBatch
}

// deletes the keys with a given prefix
func DeleteKeysWithPrefix(keyPrefix string, db badgerwrap.DB, deletionBatchSize int, numOfKeysToDelete uint64) (error, uint64, uint64) {

	// as deletion does not lock db there is a possibility that the keys for a given prefix are added while old ones are deleted. In this case it can get into a race condition.
	// In order to avoid this, count of existing keys is used which match the given prefix and deletion ends when this number of keys have been deleted

	var numOfKeysDeleted uint64 = 0
	for numOfKeysDeleted < numOfKeysToDelete {

		keysThisBatch := make([][]byte, 0, deletionBatchSize)

		// getting the keys to delete that have the given prefix
		_ = db.View(func(txn badgerwrap.Txn) error {
			iterOpt := badger.DefaultIteratorOptions
			iterOpt.PrefetchValues = false
			iterOpt.InternalAccess = true
			it := txn.NewIterator(iterOpt)
			defer it.Close()

			// TODO: Investigate if Seek() can be used instead of rewind
			for it.Rewind(); it.ValidForPrefix([]byte(keyPrefix)) || it.ValidForPrefix([]byte("!badger!move"+keyPrefix)); it.Next() {
				keyToDel := it.Item().KeyCopy(nil)
				keysThisBatch = append(keysThisBatch, keyToDel)
				if len(keysThisBatch) == deletionBatchSize {
					break
				}

			}
			return nil
		})

		// deleting the keys in batch
		if len(keysThisBatch) > 0 {
			err, deletedKeysInThisBatch := deleteKeys(db, keysThisBatch)
			numOfKeysDeleted += deletedKeysInThisBatch
			if err != nil {
				glog.Errorf("Error encountered while deleting keys with prefix: '%v', numberOfKeysDeleted: '%v' numOfKeysToDelete: '%v'", keyPrefix, numOfKeysDeleted, numOfKeysToDelete)
				return err, numOfKeysDeleted, numOfKeysToDelete
			}
		}
	}
	return nil, numOfKeysDeleted, numOfKeysToDelete
}

// returns the number of keys in DB with given prefix. If prefix is not provided it gives count of all keys
func GetTotalKeyCount(db badgerwrap.DB, keyPrefix string) uint64 {
	var totalKeyCount uint64 = 0
	keyPrefixToMatch := []byte(keyPrefix)
	_ = db.View(func(txn badgerwrap.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		if len(keyPrefixToMatch) != 0 {
			iterOpt.Prefix = keyPrefixToMatch
		}
		it := txn.NewIterator(iterOpt)
		defer it.Close()
		for it.Rewind(); it.ValidForPrefix(keyPrefixToMatch); it.Next() {
			totalKeyCount++
		}
		return nil
	})
	return totalKeyCount
}
