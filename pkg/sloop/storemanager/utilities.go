package storemanager

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func dropPrefixNoLock(keyPrefix []byte, db badgerwrap.DB) (error, float64) {
	var err error
	var allKeys [][]byte
	err = db.View(func(txn badgerwrap.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.Prefix = keyPrefix
		iterOpt.AllVersions = false
		iterOpt.PrefetchValues = false
		it := txn.NewIterator(iterOpt)
		defer it.Close()
		for it.Seek(keyPrefix); it.ValidForPrefix(keyPrefix); it.Next() {
			keyToDel := it.Item().KeyCopy(nil)
			allKeys = append(allKeys, keyToDel)
		}
		return nil
	})
	if err != nil {
		return err, 0
	}
	numOfKeysDeleted := 0
	collectSize := 100000
	keysThisBatch := [][]byte{}
	for idx, thisKey := range allKeys {
		keysThisBatch = append(keysThisBatch, thisKey)
		if len(keysThisBatch) > collectSize || idx == len(allKeys)-1 {
			err := db.Update(func(txn badgerwrap.Txn) error {
				for _, keyToDel := range keysThisBatch {
					txn.Delete(keyToDel)
				}
				return nil
			})

			if err != nil {

				return err, float64(numOfKeysDeleted)
			}

			numOfKeysDeleted += len(keysThisBatch)
			keysThisBatch = make([][]byte, 0, collectSize)
		}

	}
	return nil, float64(numOfKeysDeleted)
}
