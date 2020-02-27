/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"fmt"
	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"time"
)

// Key is /<partition>/<kind>/<namespace>/<name>/<uid>
//
// Partition is UnixSeconds rounded down to partition duration
// Kind is kubernetes kind, starts with upper case
// Namespace is kubernetes namespace, all lower
// Name is kubernetes name, all lower
// Uid is kubernetes $.metadata.uid

type ResourceSummaryKey struct {
	PartitionId string
	Kind        string
	Namespace   string
	Name        string
	Uid         string
}

func NewResourceSummaryKey(timestamp time.Time, kind string, namespace string, name string, uid string) *ResourceSummaryKey {
	partitionId := untyped.GetPartitionId(timestamp)
	return &ResourceSummaryKey{PartitionId: partitionId, Kind: kind, Namespace: namespace, Name: name, Uid: uid}
}

func NewResourceSummaryKeyComparator(kind string, namespace string, name string, uid string) *ResourceSummaryKey {
	return &ResourceSummaryKey{Kind: kind, Namespace: namespace, Name: name, Uid: uid}
}

func (*ResourceSummaryKey) TableName() string {
	return "ressum"
}

func (k *ResourceSummaryKey) Parse(key string) error {
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
func (k *ResourceSummaryKey) String() string {
	if k.Uid == "" {
		return fmt.Sprintf("/%v/%v/%v/%v/%v", k.TableName(), k.PartitionId, k.Kind, k.Namespace, k.Name)
	} else {
		return fmt.Sprintf("/%v/%v/%v/%v/%v/%v", k.TableName(), k.PartitionId, k.Kind, k.Namespace, k.Name, k.Uid)
	}
}

func (k *ResourceSummaryKey) SetPartitionId(newPartitionId string) {
	k.PartitionId = newPartitionId
}

func (*ResourceSummaryKey) ValidateKey(key string) error {
	newKey := ResourceSummaryKey{}
	return newKey.Parse(key)
}
