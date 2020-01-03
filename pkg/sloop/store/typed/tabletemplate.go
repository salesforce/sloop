/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"strconv"
	"time"
)

//go:generate genny -in=$GOFILE -out=watchtablegen.go gen "ValueType=KubeWatchResult KeyType=WatchTableKey"
//go:generate genny -in=$GOFILE -out=resourcesummarytablegen.go gen "ValueType=ResourceSummary KeyType=ResourceSummaryKey"
//go:generate genny -in=$GOFILE -out=eventcounttablegen.go gen "ValueType=ResourceEventCounts KeyType=EventCountKey"
//go:generate genny -in=$GOFILE -out=watchactivitytablegen.go gen "ValueType=WatchActivity KeyType=WatchActivityKey"

type ValueTypeTable struct {
	tableName string
}

func OpenValueTypeTable() *ValueTypeTable {
	keyInst := &KeyType{}
	return &ValueTypeTable{tableName: keyInst.TableName()}
}

func (t *ValueTypeTable) Set(txn badgerwrap.Txn, key string, value *ValueType) error {
	err := (&KeyType{}).ValidateKey(key)
	if err != nil {
		return errors.Wrapf(err, "invalid key for table %v: %v", t.tableName, key)
	}

	outb, err := proto.Marshal(value)
	if err != nil {
		return errors.Wrapf(err, "protobuf marshal for table %v failed", t.tableName)
	}

	err = txn.Set([]byte(key), outb)
	if err != nil {
		return errors.Wrapf(err, "set for table %v failed", t.tableName)
	}
	return nil
}

func (t *ValueTypeTable) Get(txn badgerwrap.Txn, key string) (*ValueType, error) {
	err := (&KeyType{}).ValidateKey(key)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid key for table %v: %v", t.tableName, key)
	}

	item, err := txn.Get([]byte(key))
	if err == badger.ErrKeyNotFound {
		// Dont wrap. Need to preserve error type
		return nil, err
	} else if err != nil {
		return nil, errors.Wrapf(err, "get failed for table %v", t.tableName)
	}

	valueBytes, err := item.ValueCopy([]byte{})
	if err != nil {
		return nil, errors.Wrapf(err, "value copy failed for table %v", t.tableName)
	}

	retValue := &ValueType{}
	err = proto.Unmarshal(valueBytes, retValue)
	if err != nil {
		return nil, errors.Wrapf(err, "protobuf unmarshal failed for table %v on value length %v", t.tableName, len(valueBytes))
	}
	return retValue, nil
}

func (t *ValueTypeTable) GetMinKey(txn badgerwrap.Txn) (bool, string) {
	keyPrefix := "/" + t.tableName + "/"
	iterOpt := badger.DefaultIteratorOptions
	iterOpt.Prefix = []byte(keyPrefix)
	iterator := txn.NewIterator(iterOpt)
	defer iterator.Close()
	iterator.Seek([]byte(keyPrefix))
	if !iterator.ValidForPrefix([]byte(keyPrefix)) {
		return false, ""
	}
	return true, string(iterator.Item().Key())
}

func (t *ValueTypeTable) GetMaxKey(txn badgerwrap.Txn) (bool, string) {
	keyPrefix := "/" + t.tableName + "/"
	iterOpt := badger.DefaultIteratorOptions
	iterOpt.Prefix = []byte(keyPrefix)
	iterOpt.Reverse = true
	iterator := txn.NewIterator(iterOpt)
	defer iterator.Close()
	// We need to seek to the end of the range so we add a 255 character at the end
	iterator.Seek([]byte(keyPrefix + string(rune(255))))
	if !iterator.Valid() {
		return false, ""
	}
	return true, string(iterator.Item().Key())
}

func (t *ValueTypeTable) GetMinMaxPartitions(txn badgerwrap.Txn) (bool, string, string) {
	ok, minKeyStr := t.GetMinKey(txn)
	if !ok {
		return false, "", ""
	}
	ok, maxKeyStr := t.GetMaxKey(txn)
	if !ok {
		// This should be impossible
		return false, "", ""
	}

	minKey := &KeyType{}
	maxKey := &KeyType{}

	err := minKey.Parse(minKeyStr)
	if err != nil {
		panic(fmt.Sprintf("invalid key in table: %v key: %q error: %v", t.tableName, minKeyStr, err))
	}

	err = maxKey.Parse(maxKeyStr)
	if err != nil {
		panic(fmt.Sprintf("invalid key in table: %v key: %q error: %v", t.tableName, maxKeyStr, err))
	}

	return true, minKey.PartitionId, maxKey.PartitionId
}

func (t *ValueTypeTable) GetUniquePartitionList(txn badgerwrap.Txn) ([]string, error) {
	resources := []string{}
	ok, minPar, maxPar := t.GetMinMaxPartitions(txn)
	if ok {
		parDuration := untyped.GetPartitionDuration()
		for curPar := minPar; curPar <= maxPar; {
			resources = append(resources, curPar)
			// update curPar
			partInt, err := strconv.ParseInt(curPar, 10, 64)
			if err != nil {
				return resources, errors.Wrapf(err, "failed to get partition:%v", curPar)
			}
			parTime := time.Unix(partInt, 0).UTC().Add(parDuration)
			curPar = untyped.GetPartitionId(parTime)
		}
	}
	return resources, nil
}

func (t *ValueTypeTable) GetPreviousKey(txn badgerwrap.Txn, key *KeyType, keyComparator *KeyType) (*KeyType, error) {
	partitionList, err := t.GetUniquePartitionList(txn)
	if err != nil {
		return &KeyType{}, errors.Wrapf(err, "failed to get partition list from table:%v", t.tableName)
	}
	currentPartition := key.PartitionId
	for i := len(partitionList) - 1; i >= 0; i-- {
		prePart := partitionList[i]
		if prePart > currentPartition {
			continue
		} else {
			prevFound, prevKey, err := t.getLastMatchingKeyInPartition(txn, prePart, key, keyComparator)
			if err != nil {
				return &KeyType{}, errors.Wrapf(err, "Failure getting previous key for %v, for partition id:%v", key.String(), prePart)
			}
			if prevFound && err == nil {
				return prevKey, nil
			}
		}
	}
	return &KeyType{}, fmt.Errorf("failed to get any previous key in table:%v, for key:%v, keyComparator:%v", t.tableName, key.String(), keyComparator)
}

func (t *ValueTypeTable) getLastMatchingKeyInPartition(txn badgerwrap.Txn, curPartition string, curKey *KeyType, keyComparator *KeyType) (bool, *KeyType, error) {
	iterOpt := badger.DefaultIteratorOptions
	iterOpt.Reverse = true
	itr := txn.NewIterator(iterOpt)
	defer itr.Close()

	oldKey := curKey.String()

	// update partition with current value
	curKey.SetPartitionId(curPartition)
	keyComparator.SetPartitionId(curPartition)

	keySeekStr := curKey.String() + string(rune(255))
	itr.Seek([]byte(keySeekStr))

	// if the result is same as key, we want to check its previous one
	if itr.Valid() && oldKey == string(itr.Item().Key()) {
		itr.Next()
	}

	if itr.ValidForPrefix([]byte(keyComparator.String())) {
		key := &KeyType{}
		err := key.Parse(string(itr.Item().Key()))
		if err != nil {
			return true, &KeyType{}, err
		}
		return true, key, nil
	}
	return false, &KeyType{}, nil
}

func (t *ValueTypeTable) RangeRead(txn badgerwrap.Txn, keyPrefix *KeyType,
	keyPredicateFn func(string) bool, valPredicateFn func(*ValueType) bool, startTime time.Time, endTime time.Time) (map[KeyType]*ValueType, RangeReadStats, error) {
	resources := map[KeyType]*ValueType{}

	stats := RangeReadStats{}
	before := time.Now()

	partitionList, err := t.GetPartitionsFromTimeRange(txn, startTime, endTime)
	stats.PartitionCount = len(partitionList)
	if err != nil {
		return resources, stats, errors.Wrapf(err, "failed to get partitions from table:%v, from startTime:%v, to endTime:%v", t.tableName, startTime, endTime)
	}

	for _, currentPartition := range partitionList {
		var seekStr string

		// when keyPrefix does not have such info as kind,namespace,and etc, we seek from /tableName/currentPartition/
		if keyPrefix == nil {
			seekStr = "/" + t.tableName + "/" + currentPartition + "/"
		} else {
			// update keyPrefix with current partition
			keyPrefix.SetPartitionId(currentPartition)
			seekStr = keyPrefix.String()
		}

		itr := txn.NewIterator(badger.IteratorOptions{Prefix: []byte(seekStr)})
		defer itr.Close()

		//in worst case, when seekStr = /table/partition, we need to iterate a key list and return all of them
		//in most cases, we should only hit one result per partition
		for itr.Seek([]byte(seekStr)); itr.ValidForPrefix([]byte(seekStr)); itr.Next() {
			stats.RowsVisitedCount += 1
			if keyPredicateFn != nil {
				if !keyPredicateFn(string(itr.Item().Key())) {
					continue
				}
			}
			key := KeyType{}
			err := key.Parse(string(itr.Item().Key()))
			if err != nil {
				return nil, stats, err
			}

			stats.RowsPassedKeyPredicateCount += 1

			valueBytes, err := itr.Item().ValueCopy([]byte{})
			if err != nil {
				return nil, stats, err
			}
			retValue := &ValueType{}
			err = proto.Unmarshal(valueBytes, retValue)
			if err != nil {
				return nil, stats, err
			}
			if valPredicateFn != nil && !valPredicateFn(retValue) {
				continue
			}
			stats.RowsPassedValuePredicateCount += 1
			resources[key] = retValue
		}

		//Close() is safe to call more than once, close at the end of each partition to avoid having old iterators open
		itr.Close()
	}

	stats.Elapsed = time.Since(before)
	stats.TableName = (&KeyType{}).TableName()
	return resources, stats, nil
}

//todo: need to add unit test
func (t *ValueTypeTable) GetPartitionsFromTimeRange(txn badgerwrap.Txn, startTime time.Time, endTime time.Time) ([]string, error) {
	resources := []string{}
	startPartition := untyped.GetPartitionId(startTime)
	endPartition := untyped.GetPartitionId(endTime)
	parDuration := untyped.GetPartitionDuration()
	for curPar := startPartition; curPar <= endPartition; {
		resources = append(resources, curPar)
		// update curPar
		partInt, err := strconv.ParseInt(curPar, 10, 64)
		if err != nil {
			return resources, errors.Wrapf(err, "failed to get partition:%v", curPar)
		}
		parTime := time.Unix(partInt, 0).UTC().Add(parDuration)
		curPar = untyped.GetPartitionId(parTime)
	}
	return resources, nil
}

func ValueType_ValPredicateFns(valFn ...func(*ValueType) bool) func(*ValueType) bool {
	return func(result *ValueType) bool {
		for _, thisFn := range valFn {
			if !thisFn(result) {
				return false
			}
		}
		return true
	}
}

func ValueType_KeyPredicateFns(keyFn ...func(string) bool) func(string) bool {
	return func(result string) bool {
		for _, thisFn := range keyFn {
			if !thisFn(result) {
				return false
			}
		}
		return true
	}
}
