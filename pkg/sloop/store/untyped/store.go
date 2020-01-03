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

func OpenStore(factory badgerwrap.Factory, rootPath string, configPartitionDuration time.Duration) (badgerwrap.DB, error) {
	err := os.MkdirAll(rootPath, 0755)
	if err != nil {
		glog.Infof("mkdir failed with %v", err)
	}
	// For now using a temp name because this all need to be replaced when we add real table/partition support
	opts := badger.DefaultOptions(rootPath)

	db, err := factory.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("badger.OpenStore failed with: %v", err)
	}

	if configPartitionDuration != time.Hour && configPartitionDuration != 24*time.Hour {
		return nil, fmt.Errorf("Only hour and day partitionDurations are supported")
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
