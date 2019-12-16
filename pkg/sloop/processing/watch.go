/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package processing

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"time"
)

func getLastKubeWatchResult(tables typed.Tables, txn badgerwrap.Txn, ts *timestamp.Timestamp, kind string, namespace string, name string) (*typed.KubeWatchResult, error) {
	keyPrefixWithoutTs, err := toWatchTableKeyPrefix(ts, kind, namespace, name)
	if err != nil {
		return nil, err
	}

	prevFound, prevKey, err := getLastWatchKey(txn, keyPrefixWithoutTs)
	if err != nil {
		return nil, errors.Wrapf(err, "Failure getting previous watch result for %v", keyPrefixWithoutTs.String())
	}
	if !prevFound {
		return nil, nil
	}

	prevWatch, err := tables.WatchTable().Get(txn, prevKey)
	if err != nil {
		return nil, err
	}
	return prevWatch, nil
}

// TODO: This code was labeled 'Previous' but really only returns 'Last', it may be helpful to actually have a 'Previous' implementation
// TODO: Move this to code-gen per table
func getLastWatchKey(txn badgerwrap.Txn, keyPrefix *typed.WatchTableKey) (bool, string, error) {
	// Retrieve the previous copy of this node and see if differences are important
	// Badger reverse seek is pretty goofy.  We need a prefix with 255 at the end for the seek, but not for prefix
	keyPrefixStr := keyPrefix.String()
	keyPrefixEndBytes := []byte(keyPrefixStr + string(rune(255)))
	keyPrefixBytes := []byte(keyPrefix.String())

	iterOpt := badger.DefaultIteratorOptions
	iterOpt.Prefix = []byte(keyPrefixBytes)
	iterOpt.Reverse = true
	itr := txn.NewIterator(iterOpt)
	defer itr.Close()
	itr.Seek([]byte(keyPrefixEndBytes))
	if itr.ValidForPrefix(keyPrefixBytes) {
		item := itr.Item()
		return true, string(item.Key()), nil
	}
	return false, "", nil
}

func doesNodeHaveMajorUpdates(tables typed.Tables, txn badgerwrap.Txn, watchRec *typed.KubeWatchResult, metadata *kubeextractor.KubeMetadata) (bool, error) {
	prevValue, err := getLastKubeWatchResult(tables, txn, watchRec.Timestamp, watchRec.Kind, metadata.Namespace, metadata.Name)
	if err != nil {
		return false, err
	}
	if prevValue == nil {
		return true, nil
	}

	diff, err := kubeextractor.NodeHasMajorUpdate(prevValue.Payload, watchRec.Payload)
	if err != nil {
		keyPrefix, _ := toWatchTableKeyPrefix(watchRec.Timestamp, watchRec.Kind, metadata.Namespace, metadata.Name)
		return false, errors.Wrapf(err, "Failed to check if nodes have meaningfully differences for %v", keyPrefix.String())
	}
	return diff, nil
}

func updateKubeWatchTable(tables typed.Tables, txn badgerwrap.Txn, watchRec *typed.KubeWatchResult, metadata *kubeextractor.KubeMetadata, keepMinorNodeUpdates bool) error {
	metricProcessingWatchtableUpdatecount.Inc()

	key, err := toWatchTableKey(watchRec.Timestamp, watchRec.Kind, metadata.Namespace, metadata.Name)
	if err != nil {
		return err
	}

	if watchRec.Kind == kubeextractor.NodeKind && !keepMinorNodeUpdates {
		hasUpdates, err := doesNodeHaveMajorUpdates(tables, txn, watchRec, metadata)
		if err != nil {
			return err
		}
		if !hasUpdates {
			glog.V(2).Infof("Not inserting node %v because it has no major updates", key.String())
			return nil
		}
	}

	err = tables.WatchTable().Set(txn, key.String(), watchRec)
	if err != nil {
		return errors.Wrap(err, "Put failed")
	}

	return nil
}

func toWatchTableKey(ts *timestamp.Timestamp, kind string, namespace string, name string) (*typed.WatchTableKey, error) {
	timestamp, err := ptypes.Timestamp(ts)
	if err != nil {
		return &typed.WatchTableKey{}, errors.Wrapf(err, "Could not convert timestamp %v", ts.String())
	}

	return typed.NewWatchTableKey(untyped.GetPartitionId(timestamp), kind, namespace, name, timestamp), nil
}

func toWatchTableKeyPrefix(ts *timestamp.Timestamp, kind string, namespace string, name string) (*typed.WatchTableKey, error) {
	timestamp, err := ptypes.Timestamp(ts)
	if err != nil {
		return &typed.WatchTableKey{}, errors.Wrapf(err, "Could not convert timestamp %v for key prefix", ts.String())
	}

	return typed.NewWatchTableKey(untyped.GetPartitionId(timestamp), kind, namespace, name, time.Time{}), nil
}
