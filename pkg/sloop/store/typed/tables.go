/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package typed

import (
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"sort"
)

type Tables interface {
	ResourceSummaryTable() *ResourceSummaryTable
	EventCountTable() *ResourceEventCountsTable
	WatchTable() *KubeWatchResultTable
	WatchActivityTable() *WatchActivityTable
	Db() badgerwrap.DB
	GetMinAndMaxPartition() (bool, string, string, error)
	GetTableNames() []string
	GetTables() []interface{}
}

type MinMaxPartitionsGetter interface {
	GetMinMaxPartitions(badgerwrap.Txn) (bool, string, string)
}

type tablesImpl struct {
	resourceSummaryTable *ResourceSummaryTable
	eventCountTable      *ResourceEventCountsTable
	watchTable           *KubeWatchResultTable
	watchActivityTable   *WatchActivityTable
	db                   badgerwrap.DB
}

func NewTableList(db badgerwrap.DB) Tables {
	t := &tablesImpl{}
	t.resourceSummaryTable = OpenResourceSummaryTable()
	t.eventCountTable = OpenResourceEventCountsTable()
	t.watchTable = OpenKubeWatchResultTable()
	t.watchActivityTable = OpenWatchActivityTable()
	t.db = db
	return t
}

func (t *tablesImpl) ResourceSummaryTable() *ResourceSummaryTable {
	return t.resourceSummaryTable
}

func (t *tablesImpl) EventCountTable() *ResourceEventCountsTable {
	return t.eventCountTable
}

func (t *tablesImpl) WatchTable() *KubeWatchResultTable {
	return t.watchTable
}

func (t *tablesImpl) WatchActivityTable() *WatchActivityTable {
	return t.watchActivityTable
}

func (t *tablesImpl) Db() badgerwrap.DB {
	return t.db
}

func (t *tablesImpl) GetMinAndMaxPartition() (bool, string, string, error) {
	allPartitions := []string{}
	err := t.db.View(func(txn badgerwrap.Txn) error {
		for _, table := range t.GetTables() {
			coerced, canCoerce := table.(MinMaxPartitionsGetter)
			if !canCoerce {
				glog.Errorf("Expected type to implement GetMinMaxPartitions but failed")
				continue
			}
			ok, minPar, maxPar := coerced.GetMinMaxPartitions(txn)
			if ok {
				allPartitions = append(allPartitions, minPar, maxPar)
			}
		}
		return nil
	})

	if err != nil {
		return false, "", "", err
	}
	if len(allPartitions) == 0 {
		return false, "", "", nil
	}

	sort.Strings(allPartitions)
	return true, allPartitions[0], allPartitions[len(allPartitions)-1], nil
}

func (t *tablesImpl) GetTableNames() []string {
	return []string{t.watchTable.tableName, t.resourceSummaryTable.tableName, t.eventCountTable.tableName, t.watchActivityTable.tableName}
}

func (t *tablesImpl) GetTables() []interface{} {
	intfs := new([]interface{})
	*intfs = append(*intfs, t.eventCountTable, t.resourceSummaryTable, t.watchTable, t.watchActivityTable)
	return *intfs
}
