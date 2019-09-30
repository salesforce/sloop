/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"strings"
	"time"
)

type EventCountKey struct {
	PartitionId string
	Kind        string
	Namespace   string
	Name        string
	Uid         string
}

func NewEventCountKey(timestamp time.Time, kind string, namespace string, name string, uid string) *EventCountKey {
	partitionId := untyped.GetPartitionId(timestamp)
	return &EventCountKey{PartitionId: partitionId, Kind: kind, Namespace: namespace, Name: name, Uid: uid}
}

func (_ *EventCountKey) TableName() string {
	return "eventcount"
}

func (k *EventCountKey) Parse(key string) error {
	parts := strings.Split(key, "/")
	if len(parts) != 7 {
		return fmt.Errorf("Key should have 6 parts: %v", key)
	}
	if parts[0] != "" {
		return fmt.Errorf("Key should start with /: %v", key)
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

func (k *EventCountKey) String() string {
	return fmt.Sprintf("/%v/%v/%v/%v/%v/%v", k.TableName(), k.PartitionId, k.Kind, k.Namespace, k.Name, k.Uid)
}

func (_ *EventCountKey) ValidateKey(key string) error {
	newKey := EventCountKey{}
	return newKey.Parse(key)
}

func (t *ResourceEventCountsTable) GetOrDefault(txn badgerwrap.Txn, key string) (*ResourceEventCounts, error) {
	rec, err := t.Get(txn, key)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return nil, err
		} else {
			return &ResourceEventCounts{MapMinToEvents: make(map[int64]*EventCounts)}, nil
		}
	}
	return rec, nil
}

func (k *EventCountKey) SetPartitionId(newPartitionId string) {
	k.PartitionId = newPartitionId
}
