package common

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

func deleteKeys(db badgerwrap.DB, keysForDelete [][]byte) (error, int) {
	deletedKeysInThisBatch := 0
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

func DeleteKeysWithPrefix(keyPrefix []byte, db badgerwrap.DB, deletionBatchSize int) (error, int, int) {
	numOfKeysToDelete := 0
	numOfKeysDeleted := 0
	keysLeftToDelete := true

	for keysLeftToDelete {

		keysThisBatch := make([][]byte, 0, deletionBatchSize)

		// getting the keys to delete that have the given prefix
		_ = db.View(func(txn badgerwrap.Txn) error {
			iterOpt := badger.DefaultIteratorOptions
			iterOpt.Prefix = keyPrefix
			iterOpt.AllVersions = false
			iterOpt.PrefetchValues = false
			it := txn.NewIterator(iterOpt)
			defer it.Close()

			for it.Seek(keyPrefix); it.ValidForPrefix(keyPrefix); it.Next() {
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
			numOfKeysToDelete += len(keysThisBatch)
			numOfKeysDeleted += deletedKeysInThisBatch
			if err != nil {
				return err, numOfKeysDeleted, numOfKeysToDelete
			}
		}

		if len(keysThisBatch) < deletionBatchSize {
			keysLeftToDelete = false
		}
	}

	return nil, numOfKeysDeleted, numOfKeysToDelete

}

func GetTotalKeyCount(db badgerwrap.DB) uint64 {
	var totalKeyCount uint64 = 0
	_ = db.View(func(txn badgerwrap.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		it := txn.NewIterator(iterOpt)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			totalKeyCount++
		}
		return nil
	})
	return totalKeyCount
}
