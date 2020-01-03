/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package untyped

import (
	"fmt"
	badger "github.com/dgraph-io/badger/v2"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"os"
	"time"
)

func OpenStore(factory badgerwrap.Factory, rootPath string, configPartitionDuration time.Duration, badgerMaxTableSize int64, badgerKeepL0InMemory bool, badgerVLogFileSize int64, badgerVLogMaxEntries uint, badgerUseLSMOnlyOptions bool) (badgerwrap.DB, error) {
	if configPartitionDuration != time.Hour && configPartitionDuration != 24*time.Hour {
		return nil, fmt.Errorf("Only hour and day partitionDurations are supported")
	}

	err := os.MkdirAll(rootPath, 0755)
	if err != nil {
		glog.Infof("mkdir failed with %v", err)
	}
	// For now using a temp name because this all need to be replaced when we add real table/partition support
	var opts badger.Options
	if badgerUseLSMOnlyOptions {
		opts = badger.LSMOnlyOptions(rootPath)
	} else {
		opts = badger.DefaultOptions(rootPath)
	}

	if badgerMaxTableSize != 0 {
		opts = opts.WithMaxTableSize(badgerMaxTableSize)
	}
	opts.KeepL0InMemory = badgerKeepL0InMemory
	if badgerVLogFileSize != 0 {
		opts = opts.WithValueLogFileSize(badgerVLogFileSize)
	}
	if badgerVLogMaxEntries != 0 {
		opts = opts.WithValueLogMaxEntries(uint32(badgerVLogMaxEntries))
	}

	db, err := factory.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("badger.OpenStore failed with: %v", err)
	}

	glog.Infof("BadgerDB Options: %+v", opts)
	lsm, vlog := db.Size()
	glog.Infof("BadgerDB Size lsm=%v, vlog=%v", lsm, vlog)
	tables := db.Tables(true)
	for _, table := range tables {
		glog.Infof("BadgerDB TABLE id=%v keycount=%v level=%v left=%v right=%v", table.ID, table.KeyCount, table.Level, string(table.Left), string(table.Right))
	}

	partitionDuration = configPartitionDuration
	return db, nil
}

func CloseStore(db badgerwrap.DB) error {
	glog.Infof("Closing store")
	err := db.Close()
	glog.Infof("Finished closing store")
	return err
}
