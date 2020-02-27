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
	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

// Key is /<partition>/<kind>/<namespace>/<name>
//
// Partition is UnixSeconds rounded down to partition duration
// Kind is kubernetes kind, starts with upper case
// Namespace is kubernetes namespace, all lower
// Name is kubernetes name, all lower

type WatchActivityKey struct {
	PartitionId string
	Kind        string
	Namespace   string
	Name        string
	Uid         string
}

func NewWatchActivityKey(partitionId string, kind string, namespace string, name string, uid string) *WatchActivityKey {
	return &WatchActivityKey{PartitionId: partitionId, Kind: kind, Namespace: namespace, Name: name, Uid: uid}
}

func NewWatchActivityKeyComparator(kind string, namespace string, name string, uid string) *WatchActivityKey {
	return &WatchActivityKey{Kind: kind, Namespace: namespace, Name: name, Uid: uid}
}

func (*WatchActivityKey) TableName() string {
	return "watchactivity"
}

func (k *WatchActivityKey) Parse(key string) error {
	err, parts := common.ParseKey(key)
	if err != nil {
		return err
	}

	if parts[1] != k.TableName() {
		return fmt.Errorf("Second part of key (%v) should be %v", key, k.TableName())
	}
	k.PartitionId = parts[2]
	k.Kind = parts[3]
	k.Namespace = parts[4]
	k.Name = parts[5]
	k.Uid = parts[6]
	return nil
}

//todo: need to make sure it can work as keyPrefix when some fields are empty
func (k *WatchActivityKey) String() string {
	return fmt.Sprintf("/%v/%v/%v/%v/%v/%v", k.TableName(), k.PartitionId, k.Kind, k.Namespace, k.Name, k.Uid)
}

func (*WatchActivityKey) ValidateKey(key string) error {
	newKey := WatchActivityKey{}
	return newKey.Parse(key)
}

func (k *WatchActivityKey) SetPartitionId(newPartitionId string) {
	k.PartitionId = newPartitionId
}

func (t *WatchActivityTable) GetOrDefault(txn badgerwrap.Txn, key string) (*WatchActivity, error) {
	rec, err := t.Get(txn, key)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return nil, err
		} else {
			return &WatchActivity{}, nil
		}
	}
	return rec, nil
}
