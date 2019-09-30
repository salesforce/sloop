/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"fmt"
	"github.com/pkg/errors"
	"strconv"
	"strings"
	"time"
)

// Key is /<partition>/<kind>/<namespace>/<name>/<timestamp>
//
// Partition is UnixSeconds rounded down to partition duration
// Kind is kubernetes kind, starts with upper case
// Namespace is kubernetes namespace, all lower
// Name is kubernetes name, all lower
// Timestamp is UnixNano in UTC

type WatchTableKey struct {
	PartitionId string
	Kind        string
	Namespace   string
	Name        string
	Timestamp   time.Time
}

func NewWatchTableKey(partitionId string, kind string, namespace string, name string, timestamp time.Time) *WatchTableKey {
	return &WatchTableKey{PartitionId: partitionId, Kind: kind, Namespace: namespace, Name: name, Timestamp: timestamp}
}

func (_ *WatchTableKey) TableName() string {
	return "watch"
}

func (k *WatchTableKey) Parse(key string) error {
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
	tsint, err := strconv.ParseInt(parts[6], 10, 64)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse timestamp from key: %v", key)
	}
	k.Timestamp = time.Unix(0, tsint).UTC()
	return nil
}

func (k *WatchTableKey) SetPartitionId(newPartitionId string) {
	k.PartitionId = newPartitionId
}

func (k *WatchTableKey) String() string {
	if k.Timestamp.IsZero() {
		return fmt.Sprintf("/%v/%v/%v/%v/%v/", k.TableName(), k.PartitionId, k.Kind, k.Namespace, k.Name)
	} else {
		return fmt.Sprintf("/%v/%v/%v/%v/%v/%v", k.TableName(), k.PartitionId, k.Kind, k.Namespace, k.Name, k.Timestamp.UnixNano())
	}
}

func (_ *WatchTableKey) ValidateKey(key string) error {
	newKey := WatchTableKey{}
	return newKey.Parse(key)
}
