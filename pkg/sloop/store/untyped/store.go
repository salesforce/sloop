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

type Config struct {
	RootPath                 string
	ConfigPartitionDuration  time.Duration
	BadgerMaxTableSize       int64
	BadgerKeepL0InMemory     bool
	BadgerVLogFileSize       int64
	BadgerVLogMaxEntries     uint
	BadgerUseLSMOnlyOptions  bool
	BadgerEnableEventLogging bool
	BadgerNumOfCompactors    int
	BadgerNumL0Tables        int
	BadgerNumL0TablesStall   int
	BadgerSyncWrites         bool
	BadgerLevelOneSize       int64
	BadgerLevSizeMultiplier  int
}

func OpenStore(factory badgerwrap.Factory, config *Config) (badgerwrap.DB, error) {
	if config.ConfigPartitionDuration != time.Hour && config.ConfigPartitionDuration != 24*time.Hour {
		return nil, fmt.Errorf("Only hour and day partitionDurations are supported")
	}

	err := os.MkdirAll(config.RootPath, 0755)
	if err != nil {
		glog.Infof("mkdir failed with %v", err)
	}

	var opts badger.Options
	if config.BadgerUseLSMOnlyOptions {
		// LSMOnlyOptions uses less disk space for vlog files.  See the comments on the LSMOnlyOptions() func for details
		opts = badger.LSMOnlyOptions(config.RootPath)
	} else {
		opts = badger.DefaultOptions(config.RootPath)
	}

	if config.BadgerEnableEventLogging {
		opts = opts.WithEventLogging(true)
	}

	if config.BadgerMaxTableSize != 0 {
		opts = opts.WithMaxTableSize(config.BadgerMaxTableSize)
	}
	opts.KeepL0InMemory = config.BadgerKeepL0InMemory
	if config.BadgerVLogFileSize != 0 {
		opts = opts.WithValueLogFileSize(config.BadgerVLogFileSize)
	}
	if config.BadgerVLogMaxEntries != 0 {
		opts = opts.WithValueLogMaxEntries(uint32(config.BadgerVLogMaxEntries))
	}

	if config.BadgerNumOfCompactors != 0 {
		opts = opts.WithNumCompactors(config.BadgerNumOfCompactors)
	}

	if config.BadgerNumL0Tables != 0 {
		opts = opts.WithNumLevelZeroTables(config.BadgerNumL0Tables)
	}

	if config.BadgerNumL0TablesStall != 0 {
		opts = opts.WithNumLevelZeroTablesStall(config.BadgerNumL0TablesStall)
	}

	if config.BadgerLevelOneSize != 0 {
		opts = opts.WithLevelOneSize(config.BadgerLevelOneSize)
	}

	if config.BadgerLevSizeMultiplier != 0 {
		opts = opts.WithLevelSizeMultiplier(config.BadgerLevSizeMultiplier)
	}

	opts = opts.WithSyncWrites(config.BadgerSyncWrites)

	db, err := factory.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("badger.OpenStore failed with: %v", err)
	}

	glog.Infof("BadgerDB Options: %+v", opts)

	partitionDuration = config.ConfigPartitionDuration
	return db, nil
}

func CloseStore(db badgerwrap.DB) error {
	glog.Infof("Closing store")
	err := db.Close()
	glog.Infof("Finished closing store")
	return err
}
