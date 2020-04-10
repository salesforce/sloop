package common

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/glog"
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

// deletes the keys with a given prefix
func DeleteKeysWithPrefix(keyPrefix []byte, db badgerwrap.DB, deletionBatchSize int) (error, int, int) {

	// as deletion does not lock db there is a possibility that the keys for a given prefix are added while old ones are deleted. In this case it can get into a race condition.
	// In order to avoid this, count of existing keys is taken which match the given prefix and deletion ends when this number of keys have been deleted
	numOfKeysToDelete := int(GetTotalKeyCount(db, keyPrefix))
	numOfKeysDeleted := 0

	for numOfKeysDeleted < numOfKeysToDelete {

		keysThisBatch := make([][]byte, 0, deletionBatchSize)

		// getting the keys to delete that have the given prefix
		_ = db.View(func(txn badgerwrap.Txn) error {
			iterOpt := badger.DefaultIteratorOptions
			iterOpt.Prefix = keyPrefix
			iterOpt.PrefetchValues = false
			it := txn.NewIterator(iterOpt)
			defer it.Close()

			for it.Rewind(); it.ValidForPrefix(keyPrefix); it.Next() {
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
				return err, numOfKeysDeleted, numOfKeysToDelete
			}
		}
	}
	return nil, numOfKeysDeleted, numOfKeysToDelete
}

// returns the number of keys in DB with given prefix. If prefix is not provided it gives count of all keys
func GetTotalKeyCount(db badgerwrap.DB, keyPrefix []byte) uint64 {
	var totalKeyCount uint64 = 0
	_ = db.View(func(txn badgerwrap.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		if len(keyPrefix) != 0 {
			iterOpt.Prefix = keyPrefix
		}
		it := txn.NewIterator(iterOpt)
		defer it.Close()
		for it.Rewind(); it.ValidForPrefix(keyPrefix); it.Next() {
			totalKeyCount++
		}
		return nil
	})
	return totalKeyCount
}

type SloopKey struct {
	TableName   string
	PartitionID string
}

// returns TableName, PartitionId, error.
func ParseSloopKey(item badgerwrap.Item) (string, string, error) {
	key := item.Key()
	err, parts := ParseKey(string(key))
	if err != nil {
		return "", "", err
	}

	var tableName = parts[1]
	var partitionId = parts[2]
	return tableName, partitionId, nil
}

// prints all the keys histogram. It can help debugging when needed.
func PrintKeyHistogram(db badgerwrap.DB) {
	keyHistogram := make(map[SloopKey]uint64)
	_ = db.View(func(txn badgerwrap.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		it := txn.NewIterator(iterOpt)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			tableName, partitionId, err := ParseSloopKey(item)
			if err != nil {
				glog.Infof("failed to parse information about key: %x", item.Key())
			}

			sloopKey := SloopKey{tableName, partitionId}
			keyHistogram[sloopKey]++
		}
		return nil
	})

	for key, element := range keyHistogram {
		glog.Infof("TableName: %v, PartitionId: %v, keyCount: %v", key.TableName, key.PartitionID, element)
	}
}

func GetPartitions(db badgerwrap.DB) map[string]uint64 {
	partitionHistogram := make(map[string]uint64)
	_ = db.View(func(txn badgerwrap.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		it := txn.NewIterator(iterOpt)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			_, partitionId, err := ParseSloopKey(item)
			if err != nil {
				glog.Infof("failed to parse information about key: %x", item.Key())
			}
			partitionHistogram[partitionId]++
		}
		return nil
	})

	return partitionHistogram
}
