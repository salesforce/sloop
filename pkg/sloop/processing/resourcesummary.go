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
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"time"
)

// TODO: Split this up and add unit tests

func updateResourceSummaryTable(tables typed.Tables, txn badgerwrap.Txn, watchRec *typed.KubeWatchResult, metadata *kubeextractor.KubeMetadata) error {
	if watchRec.Kind == kubeextractor.EventKind {
		glog.V(2).Infof("Skipping resource summary table update as kubewatch result is an event(selfLink: %v)", metadata.SelfLink)
		return nil
	}
	ts, err := ptypes.Timestamp(watchRec.Timestamp)
	if err != nil {
		return errors.Wrap(err, "could not convert timestamp")
	}

	key := typed.NewResourceSummaryKey(ts, watchRec.Kind, metadata.Namespace, metadata.Name, metadata.Uid).String()

	value, err := getResourceSummaryValue(tables, txn, key, metadata, watchRec)
	if err != nil {
		return errors.Wrapf(err, "could not get record for key %v", key)
	}

	value.Relationships = getRelationships(ts, metadata)

	err = tables.ResourceSummaryTable().Set(txn, key, value)
	if err != nil {
		return errors.Wrapf(err, "put for the key %v failed", key)
	}

	return nil
}

func getResourceSummaryValue(tables typed.Tables, txn badgerwrap.Txn, key string, metadata *kubeextractor.KubeMetadata, watchRec *typed.KubeWatchResult) (*typed.ResourceSummary, error) {
	value, err := tables.ResourceSummaryTable().Get(txn, key)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return nil, errors.Wrap(err, "could not get record")
		}
		createTimeProto, err := typed.StringToProtobufTimestamp(metadata.CreationTimestamp)
		if err != nil {
			return nil, errors.Wrap(err, "could not convert string to timestamp")
		}
		value = &typed.ResourceSummary{
			FirstSeen:    watchRec.Timestamp,
			CreateTime:   createTimeProto,
			DeletedAtEnd: false}
	} else if watchRec.WatchType == typed.KubeWatchResult_ADD {
		value.FirstSeen = watchRec.Timestamp
	}
	value.LastSeen = watchRec.Timestamp
	if watchRec.WatchType == typed.KubeWatchResult_DELETE {
		value.DeletedAtEnd = true
	}
	return value, nil
}

func getRelationships(timestamp time.Time, metadata *kubeextractor.KubeMetadata) []string {
	relationships := []string{}
	for _, value := range metadata.OwnerReferences {
		refKey := typed.NewResourceSummaryKey(timestamp, value.Kind, metadata.Namespace, value.Name, value.Uid).String()
		relationships = append(relationships, refKey)
	}
	return relationships
}
