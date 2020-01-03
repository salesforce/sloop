/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package processing

import (
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/stretchr/testify/assert"
)

const someNodePayload1 = `{
  "metadata": {
    "name": "someName",
    "namespace": "someNamespace",
    "resourceVersion": "456"
  },
  "status": {
    "conditions": [
      {
        "type": "OutOfDisk",
        "status": "False",
        "lastHeartbeatTime": "2012-01-01T15:35:56Z",
        "lastTransitionTime": "2019-07-19T15:35:56Z",
        "reason": "KubeletHasSufficientDisk"
      }
    ]
  }
}`
const someNodePayload2 = `{
  "metadata": {
    "name": "someName",
    "namespace": "someNamespace",
    "resourceVersion": "457"
  },
  "status": {
    "conditions": [
      {
        "type": "OutOfDisk",
        "status": "False",
        "lastHeartbeatTime": "2012-01-01T15:35:56Z",
        "lastTransitionTime": "2019-07-19T15:35:56Z",
        "reason": "KubeletHasSufficientDisk"
      }
    ]
  }
}`

func Test_updateWatchActivityTable(t *testing.T) {

	untyped.TestHookSetPartitionDuration(time.Hour)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	ts, err := ptypes.TimestampProto(someWatchTime)
	assert.Nil(t, err)
	watchRec := &typed.KubeWatchResult{Kind: kubeextractor.NodeKind, WatchType: typed.KubeWatchResult_UPDATE, Timestamp: ts, Payload: someNodePayload1}
	metadata, err := kubeextractor.ExtractMetadata(watchRec.Payload)
	assert.Nil(t, err)

	// add a WatchActivity (no matching KubeWatchResult) => no change
	err = tables.Db().Update(func(txn badgerwrap.Txn) error {
		err = updateWatchActivityTable(tables, txn, watchRec, &metadata)
		assert.Nil(t, err)

		activityRecord, _, err := getWatchActivity(tables, txn, someWatchTime, watchRec, &metadata)
		assert.Nil(t, err)
		assert.NotNil(t, activityRecord)
		assert.Equal(t, 0, len(activityRecord.ChangedAt))
		assert.Equal(t, 1, len(activityRecord.NoChangeAt))
		assert.Equal(t, someWatchTime.Unix(), activityRecord.NoChangeAt[0])

		return nil
	})
	assert.Nil(t, err)

	// add a KubeWatchResult
	err = tables.Db().Update(func(txn badgerwrap.Txn) error {
		return updateKubeWatchTable(tables, txn, watchRec, &metadata, true)
	})
	assert.Nil(t, err)

	// add a WatchActivity => no change at timestamp
	timestamp2 := someWatchTime.Add(time.Minute)
	ts2, err := ptypes.TimestampProto(timestamp2)
	assert.Nil(t, err)
	watchRec.Timestamp = ts2
	err = tables.Db().Update(func(txn badgerwrap.Txn) error {
		err = updateWatchActivityTable(tables, txn, watchRec, &metadata)
		assert.Nil(t, err)

		activityRecord, _, err := getWatchActivity(tables, txn, timestamp2, watchRec, &metadata)
		assert.Nil(t, err)
		assert.NotNil(t, activityRecord)
		assert.Equal(t, 0, len(activityRecord.ChangedAt))
		assert.Equal(t, 2, len(activityRecord.NoChangeAt))
		assert.Equal(t, timestamp2.Unix(), activityRecord.NoChangeAt[1])

		return nil
	})
	assert.Nil(t, err)

	// add a changed WatchActivity => changed at timestamp
	watchRec.Payload = someNodePayload2
	metadata, err = kubeextractor.ExtractMetadata(watchRec.Payload)
	assert.Nil(t, err)
	err = tables.Db().Update(func(txn badgerwrap.Txn) error {
		err = updateWatchActivityTable(tables, txn, watchRec, &metadata)
		assert.Nil(t, err)

		activityRecord, _, err := getWatchActivity(tables, txn, timestamp2, watchRec, &metadata)
		assert.Nil(t, err)
		assert.NotNil(t, activityRecord)
		assert.Equal(t, 1, len(activityRecord.ChangedAt))
		assert.Equal(t, 2, len(activityRecord.NoChangeAt))
		assert.Equal(t, timestamp2.Unix(), activityRecord.ChangedAt[0])

		return nil
	})
	assert.Nil(t, err)
}
