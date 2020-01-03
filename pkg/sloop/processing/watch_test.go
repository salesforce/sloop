/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package processing

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/salesforce/sloop/pkg/sloop/test/assertex"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var someKind = "Pod"
var someWatchTime = time.Date(2019, 3, 4, 3, 4, 5, 6, time.UTC)

const somePodPayload = `{
  "metadata": {
    "name": "someName",
    "namespace": "someNamespace",
    "uid": "6c2a9795-a282-11e9-ba2f-14187761de09",
    "creationTimestamp": "2019-07-09T19:47:45Z"
  }
}`
const expectedKey = "/watch/001551668400/Pod/someNamespace/someName/1551668645000000006"

const someNode = `{
  "metadata": {
    "name": "somehostname",
    "resourceVersion": "123"
  },
  "status": {
    "conditions": [
      {
        "type": "OutOfDisk",
        "status": "False",
        "lastHeartbeatTime": "2019-07-19T15:35:56Z",
        "lastTransitionTime": "2019-07-19T15:35:56Z",
        "reason": "KubeletHasSufficientDisk"
      }
    ]
  }
}`
const someNodeDiffTsAndRV = `{
  "metadata": {
    "name": "somehostname",
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
const someNodeDiffStatus = `{
  "metadata": {
    "name": "somehostname",
    "resourceVersion": "123"
  },
  "status": {
    "conditions": [
      {
        "type": "OutOfDisk",
        "status": "True",
        "lastHeartbeatTime": "2019-07-19T15:35:56Z",
        "lastTransitionTime": "2019-07-19T15:35:56Z",
        "reason": "KubeletHasSufficientDisk"
      }
    ]
  }
}`

type wtKeyValPair struct {
	Key   string
	Value *typed.KubeWatchResult
}

func helper_runWatchTableProcessingOnInputs(t *testing.T, inRecs []*typed.KubeWatchResult, keepMinorNodeUpdates bool) []wtKeyValPair {
	untyped.TestHookSetPartitionDuration(time.Hour)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	for _, watchRec := range inRecs {
		err = tables.Db().Update(func(txn badgerwrap.Txn) error {
			kubeMetadata, err := kubeextractor.ExtractMetadata(watchRec.Payload)
			assert.Nil(t, err)

			return updateKubeWatchTable(tables, txn, watchRec, &kubeMetadata, keepMinorNodeUpdates)
		})
		assert.Nil(t, err)
	}

	var foundRows []wtKeyValPair
	err = tables.Db().View(func(txn badgerwrap.Txn) error {
		itr := txn.NewIterator(badger.DefaultIteratorOptions)
		defer itr.Close()
		for itr.Rewind(); itr.Valid(); itr.Next() {
			thisKey := string(itr.Item().Key())
			thisVal, err := tables.WatchTable().Get(txn, thisKey)
			assert.Nil(t, err)
			newRows := wtKeyValPair{Key: string(itr.Item().Key()), Value: thisVal}
			foundRows = append(foundRows, newRows)
		}
		return nil
	})
	assert.Nil(t, err)

	return foundRows
}

func Test_WatchTable_BasicAddWorks(t *testing.T) {
	ts, err := ptypes.TimestampProto(someWatchTime)
	assert.Nil(t, err)
	watchRec := &typed.KubeWatchResult{Kind: someKind, WatchType: typed.KubeWatchResult_ADD, Timestamp: ts, Payload: somePodPayload}

	results := helper_runWatchTableProcessingOnInputs(t, []*typed.KubeWatchResult{watchRec}, false)

	assert.Equal(t, 1, len(results))
	actualkey := results[0].Key
	actualVal := results[0].Value
	assert.Equal(t, expectedKey, actualkey)
	assert.Equal(t, someKind, actualVal.Kind)
	assertex.ProtoEqual(t, ts, actualVal.Timestamp)
	assert.Equal(t, somePodPayload, actualVal.Payload)
	assert.Equal(t, typed.KubeWatchResult_ADD, actualVal.WatchType)
}

func Test_WatchTable_AddSameWatchResultTwiceSameTS_OneOutputRow(t *testing.T) {
	ts, err := ptypes.TimestampProto(someWatchTime)
	assert.Nil(t, err)
	watchRec := &typed.KubeWatchResult{Kind: someKind, WatchType: typed.KubeWatchResult_ADD, Timestamp: ts, Payload: somePodPayload}

	results := helper_runWatchTableProcessingOnInputs(t, []*typed.KubeWatchResult{watchRec, watchRec}, false)

	assert.Equal(t, 1, len(results))
}

func Test_WatchTable_AddSameWatchResultTwiceDiffTS_TwoOutputRows(t *testing.T) {
	ts1, err := ptypes.TimestampProto(someWatchTime)
	assert.Nil(t, err)
	ts2, err := ptypes.TimestampProto(someWatchTime.Add(time.Second))
	assert.Nil(t, err)
	watchRec1 := &typed.KubeWatchResult{Kind: someKind, WatchType: typed.KubeWatchResult_ADD, Timestamp: ts1, Payload: somePodPayload}
	watchRec2 := &typed.KubeWatchResult{Kind: someKind, WatchType: typed.KubeWatchResult_ADD, Timestamp: ts2, Payload: somePodPayload}

	results := helper_runWatchTableProcessingOnInputs(t, []*typed.KubeWatchResult{watchRec1, watchRec2}, false)

	assert.Equal(t, 2, len(results))
}

func Test_WatchTable_DontKeepMinorNodeUpdates_AddNodeTwiceOnlyDiffIsTS_OneOutputRow(t *testing.T) {
	ts1, err := ptypes.TimestampProto(someWatchTime)
	assert.Nil(t, err)
	ts2, err := ptypes.TimestampProto(someWatchTime.Add(time.Second))
	assert.Nil(t, err)
	watchRec1 := &typed.KubeWatchResult{Kind: kubeextractor.NodeKind, WatchType: typed.KubeWatchResult_UPDATE, Timestamp: ts1, Payload: someNode}
	watchRec2 := &typed.KubeWatchResult{Kind: kubeextractor.NodeKind, WatchType: typed.KubeWatchResult_UPDATE, Timestamp: ts2, Payload: someNodeDiffTsAndRV}

	results := helper_runWatchTableProcessingOnInputs(t, []*typed.KubeWatchResult{watchRec1, watchRec2}, false)

	assert.Equal(t, 1, len(results))
}

func Test_WatchTable_DontKeepMinorNodeUpdates_AddNodeTwiceWithStatusDiff_TwoOutputRows(t *testing.T) {
	ts1, err := ptypes.TimestampProto(someWatchTime)
	assert.Nil(t, err)
	ts2, err := ptypes.TimestampProto(someWatchTime.Add(time.Second))
	assert.Nil(t, err)
	watchRec1 := &typed.KubeWatchResult{Kind: kubeextractor.NodeKind, WatchType: typed.KubeWatchResult_UPDATE, Timestamp: ts1, Payload: someNode}
	watchRec2 := &typed.KubeWatchResult{Kind: kubeextractor.NodeKind, WatchType: typed.KubeWatchResult_UPDATE, Timestamp: ts2, Payload: someNodeDiffStatus}

	results := helper_runWatchTableProcessingOnInputs(t, []*typed.KubeWatchResult{watchRec1, watchRec2}, false)

	assert.Equal(t, 2, len(results))
}

func Test_WatchTable_KeepMinorNodeUpdates_AddNodeTwiceOnlyDiffIsTS_TwoOutputRow(t *testing.T) {
	ts1, err := ptypes.TimestampProto(someWatchTime)
	assert.Nil(t, err)
	ts2, err := ptypes.TimestampProto(someWatchTime.Add(time.Second))
	assert.Nil(t, err)
	watchRec1 := &typed.KubeWatchResult{Kind: kubeextractor.NodeKind, WatchType: typed.KubeWatchResult_UPDATE, Timestamp: ts1, Payload: someNode}
	watchRec2 := &typed.KubeWatchResult{Kind: kubeextractor.NodeKind, WatchType: typed.KubeWatchResult_UPDATE, Timestamp: ts2, Payload: someNodeDiffTsAndRV}

	results := helper_runWatchTableProcessingOnInputs(t, []*typed.KubeWatchResult{watchRec1, watchRec2}, true)

	assert.Equal(t, 2, len(results))
}

func Test_getLastKubeWatchResult(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	tables := typed.NewTableList(db)

	ts, err := ptypes.TimestampProto(someWatchTime)
	assert.Nil(t, err)

	watchRec := typed.KubeWatchResult{Kind: kubeextractor.NodeKind, WatchType: typed.KubeWatchResult_UPDATE, Timestamp: ts, Payload: somePodPayload}
	metadata := &kubeextractor.KubeMetadata{Name: "someName", Namespace: "someNamespace"}
	err = tables.Db().Update(func(txn badgerwrap.Txn) error {
		return updateKubeWatchTable(tables, txn, &watchRec, metadata, true)
	})
	assert.Nil(t, err)

	err = tables.Db().View(func(txn badgerwrap.Txn) error {
		prevWatch, err := getLastKubeWatchResult(tables, txn, ts, kubeextractor.NodeKind, metadata.Namespace, "differentName")
		assert.Nil(t, err)
		assert.Nil(t, prevWatch)

		prevWatch, err = getLastKubeWatchResult(tables, txn, ts, kubeextractor.NodeKind, metadata.Namespace, metadata.Name)
		assert.Nil(t, err)
		assert.NotNil(t, prevWatch)

		return nil
	})
	assert.Nil(t, err)
}
