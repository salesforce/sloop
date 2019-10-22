/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"github.com/golang/glog"
	"time"
)

// The code in this file is simply here to let us compile tabletemplate.go but these are
// things we dont want in the generated output as they would conflict with functions on the real value and key types

type ValueType struct {
}

func (p *ValueType) Reset() {
}

func (p *ValueType) String() string {
	return ""
}

func (p *ValueType) ProtoMessage() {
}

type KeyType struct {
	PartitionId string
}

func (*KeyType) ValidateKey(key string) error {
	panic("Placeholder key type should never be used")
}

func (*KeyType) TableName() string {
	panic("Placeholder key should not be used")
}

func (*KeyType) Parse(key string) error {
	panic("Placeholder key should not be used")
}

func (*KeyType) GetTestKey() string {
	panic("Placeholder key should not be used")
}

func (*KeyType) SetTestKeys() []string {
	panic("Placeholder key should not be used")
}

func (*KeyType) String() string {
	panic("Placeholder key should not be used")
}

func (*KeyType) GetTestValue() *ValueType {
	panic("Placeholder key should not be used")
}

func (p *KeyType) SetTestValue() *ValueType {
	return &ValueType{}
}

func (*KeyType) SetPartitionId(newPartitionId string) {
	panic("Placeholder key should not be used")
}

type RangeReadStats struct {
	TableName                     string
	PartitionCount                int
	RowsVisitedCount              int
	RowsPassedKeyPredicateCount   int
	RowsPassedValuePredicateCount int
	Elapsed                       time.Duration
}

func (stats RangeReadStats) Log(requestId string) {
	glog.Infof("reqId: %v range read on table %v took %v.  Partitions scanned %v.  Rows scanned %v, past key predicate %v, past value predicate %v",
		requestId, stats.TableName, stats.Elapsed, stats.PartitionCount, stats.RowsVisitedCount, stats.RowsPassedKeyPredicateCount, stats.RowsPassedValuePredicateCount)
}
