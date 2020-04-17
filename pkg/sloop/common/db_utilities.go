package common

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"sort"
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
			iterOpt.Prefix = []byte(keyPrefix)
			iterOpt.PrefetchValues = false
			it := txn.NewIterator(iterOpt)
			defer it.Close()

			for it.Rewind(); it.ValidForPrefix([]byte(keyPrefix)); it.Next() {
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
func GetPartitionIDAndTableName(item badgerwrap.Item) (string, string, error) {
	key := item.Key()
	err, parts := ParseKey(string(key))
	if err != nil {
		return "", "", err
	}

	var tableName = parts[1]
	var partitionId = parts[2]
	return tableName, partitionId, nil
}

type PartitionInfo struct {
	TotalKeyCount          uint64
	TableNameToKeyCountMap map[string]uint64
}

// prints all the keys histogram. It can help debugging when needed.
func PrintKeyHistogram(db badgerwrap.DB) {
	partitionTableNameToKeyCountMap, totalKeyCount := GetPartitionsInfo(db)
	glog.Infof("TotalkeyCount: %v", totalKeyCount)

	for partitionID, partitionInfo := range partitionTableNameToKeyCountMap {
		for tableName, keyCount := range partitionInfo.TableNameToKeyCountMap {
			glog.Infof("TableName: %v, PartitionId: %v, keyCount: %v", tableName, partitionID, keyCount)
		}
	}
}

// Returns the sorted list of partitionIDs from the given partitions Info map
func GetSortedPartitionIDs(partitionsInfoMap map[string]*PartitionInfo) []string {
	var sortedListOfPartitionIds []string

	for partitionID, _ := range partitionsInfoMap {
		sortedListOfPartitionIds = append(sortedListOfPartitionIds, partitionID)
	}

	// Sorted numbered strings here is ok since they are all of the same length
	sort.Strings(sortedListOfPartitionIds)
	return sortedListOfPartitionIds
}

// Gets the Information for partitions to key Count Map
// Returns Partitions to KeyCount Map, Partitions TableName to Key Count and total key count
func GetPartitionsInfo(db badgerwrap.DB) (map[string]*PartitionInfo, uint64) {
	var totalKeyCount uint64 = 0
	partitionIDToPartitionInfoMap := make(map[string]*PartitionInfo)

	_ = db.View(func(txn badgerwrap.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		it := txn.NewIterator(iterOpt)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			tableName, partitionId, err := GetPartitionIDAndTableName(item)
			if err != nil {
				glog.Infof("failed to parse information about key: %x", item.Key())
			}
			sloopKey := SloopKey{tableName, partitionId}
			if partitionIDToPartitionInfoMap[sloopKey.PartitionID] == nil {
				partitionIDToPartitionInfoMap[sloopKey.PartitionID] = &PartitionInfo{0, make(map[string]uint64)}
			}

			partitionIDToPartitionInfoMap[sloopKey.PartitionID].TotalKeyCount++
			partitionIDToPartitionInfoMap[sloopKey.PartitionID].TableNameToKeyCountMap[sloopKey.TableName]++
			totalKeyCount++
		}
		return nil
	})

	return partitionIDToPartitionInfoMap, totalKeyCount
}
