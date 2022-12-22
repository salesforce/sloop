package common

import (
	"sort"

	"github.com/dgraph-io/badger/v2"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

type SloopKey struct {
	TableName   string
	PartitionID string
}

// returns TableName, PartitionId, error.
func GetSloopKey(item badgerwrap.Item) (SloopKey, error) {
	key := item.Key()
	err, parts := ParseKey(string(key))
	if err != nil {
		return SloopKey{}, err
	}

	var tableName = parts[1]
	var partitionId = parts[2]
	return SloopKey{tableName, partitionId}, nil
}

type PartitionInfo struct {
	TotalKeyCount          uint64
	TableNameToKeyCountMap map[string]uint64
}

// prints all the keys histogram. It can help debugging when needed.
func PrintKeyHistogram(db badgerwrap.DB) {
	partitionTableNameToKeyCountMap, totalKeyCount := GetPartitionsInfo(db)
	glog.V(2).Infof("TotalkeyCount: %v", totalKeyCount)

	for partitionID, partitionInfo := range partitionTableNameToKeyCountMap {
		for tableName, keyCount := range partitionInfo.TableNameToKeyCountMap {
			glog.V(2).Infof("TableName: %v, PartitionId: %v, keyCount: %v", tableName, partitionID, keyCount)
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
			sloopKey, err := GetSloopKey(item)
			if err != nil {
				glog.Errorf("failed to parse information about key: %x", item.Key())
				continue
			}
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

// Return all keys within a partition with the given keyPrefix
func GetKeysForPrefix(db badgerwrap.DB, keyPrefix string) []string {
	var keys []string
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
			keys = append(keys, string(it.Item().Key()))
		}
		return nil
	})
	return keys
}
