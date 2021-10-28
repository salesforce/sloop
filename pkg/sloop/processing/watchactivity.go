/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package processing

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"time"
)

func updateWatchActivityTable(tables typed.Tables, txn badgerwrap.Txn, watchRec *typed.KubeWatchResult, metadata *kubeextractor.KubeMetadata) error {

	if watchRec.Kind == kubeextractor.EventKind {
		return nil
	}

	resourceChanged, err := didKubeWatchResultChange(tables, txn, watchRec, metadata)
	if err != nil {
		return err
	}

	timestamp, err := ptypes.Timestamp(watchRec.Timestamp)
	if err != nil {
		return errors.Wrapf(err, "Could not convert timestamp %v", watchRec.Timestamp)
	}

	activityRecord, key, err := getWatchActivity(tables, txn, timestamp, watchRec, metadata)
	if err != nil {
		return err
	}

	if resourceChanged {
		activityRecord.ChangedAt = append(activityRecord.ChangedAt, timestamp.Unix())
	} else {
		activityRecord.NoChangeAt = append(activityRecord.NoChangeAt, timestamp.Unix())
	}

	metricIngestionSuccessCount.Inc()
	return putWatchActivity(tables, txn, activityRecord, key)
}

func didKubeWatchResultChange(tables typed.Tables, txn badgerwrap.Txn, watchRec *typed.KubeWatchResult, metadata *kubeextractor.KubeMetadata) (bool, error) {
	resourceChanged := false
	prevWatch, err := getLastKubeWatchResult(tables, txn, watchRec.Timestamp, watchRec.Kind, metadata.Namespace, metadata.Name)
	if err != nil {
		return false, errors.Wrap(err, "Could not get event info for previous event instance")
	}

	if prevWatch != nil {
		prevMetadata, err := kubeextractor.ExtractMetadata(prevWatch.Payload)
		if err != nil {
			return false, errors.Wrap(err, "Cannot extract resource metadata")
		}

		resourceChanged = metadata.ResourceVersion != prevMetadata.ResourceVersion
	}

	return resourceChanged, nil
}

func getWatchActivity(tables typed.Tables, txn badgerwrap.Txn, timestamp time.Time, watchRec *typed.KubeWatchResult, metadata *kubeextractor.KubeMetadata) (*typed.WatchActivity, *typed.WatchActivityKey, error) {
	partitionId := untyped.GetPartitionId(timestamp)
	key := typed.NewWatchActivityKey(partitionId, watchRec.Kind, metadata.Namespace, metadata.Name, metadata.Uid)

	activityRecord, err := tables.WatchActivityTable().GetOrDefault(txn, key.String())
	if err != nil {
		return nil, nil, errors.Wrap(err, "Could not get watch activity record")
	}

	return activityRecord, key, nil
}

func putWatchActivity(tables typed.Tables, txn badgerwrap.Txn, activityRecord *typed.WatchActivity, key *typed.WatchActivityKey) error {
	err := tables.WatchActivityTable().Set(txn, key.String(), activityRecord)
	if err != nil {
		return errors.Wrap(err, "Failed to put watch activity record")
	}

	return nil
}
