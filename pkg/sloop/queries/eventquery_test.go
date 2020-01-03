/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/salesforce/sloop/pkg/sloop/test/assertex"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const someRequestId = "someReqId"

func helper_get_k8Watchtable(keys []string, t *testing.T, somePyaload string) typed.Tables {
	if len(somePyaload) == 0 {
		somePyaload = `{
  "reason":"failed",
  "firstTimestamp": "2019-08-29T21:24:55Z",
  "lastTimestamp": "2019-08-29T21:27:55Z",
  "count": 10}`
	}
	val := &typed.KubeWatchResult{Kind: "someKind", Payload: somePyaload}
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wt := typed.OpenKubeWatchResultTable()

	err = db.Update(func(txn badgerwrap.Txn) error {
		for _, key := range keys {
			txerr := wt.Set(txn, key, val)
			if txerr != nil {
				return txerr
			}
		}
		return nil
	})
	assert.Nil(t, err)
	tables := typed.NewTableList(db)
	return tables
}

func Test_GetEventData_False(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()
	values[KindParam] = []string{"someKind"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	var keys []string
	for i := 'a'; i < 'd'; i++ {
		keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind"+string(i), "someNamespace", "someName.xx", someTs).String())
	}
	starTime := someTs.Add(-60 * time.Minute)
	endTime := someTs.Add(60 * time.Minute)
	tables := helper_get_k8Watchtable(keys, t, "")
	res, err := GetEventData(values, tables, starTime, endTime, someRequestId)
	assert.Equal(t, string(res), "")
	assert.Nil(t, err)
}

func Test_GetEventData_KeyNotInTimeRange(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()
	values[KindParam] = []string{"someKinda"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	var keys []string
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKinda", "someNamespace", "someName.xx", someTs).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKinda", "someNamespace", "someName.xx", someTs.Add(-10*time.Minute)).String())
	for i := 'b'; i < 'd'; i++ {
		keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind"+string(i), "someNamespace", "someName.xx", someTs).String())
	}
	tables := helper_get_k8Watchtable(keys, t, "")
	res, err := GetEventData(values, tables, someTs.Add(-60*time.Minute), someTs.Add(60*time.Minute), someRequestId)
	assert.Nil(t, err)
	assert.Equal(t, string(res), "")
}

func Test_GetEventData_True(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()
	values[KindParam] = []string{"someKinda"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	var keys []string
	keys = append(keys, typed.NewWatchTableKey(partitionId, "Event", "someNamespace", "someName.xx", someTs).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, "Event", "someNamespaceb", "someName.xx", someTs).String())
	someEventPayload := `{
  "involvedObject": {
    "kind": "someKinda",
    "namespace": "user-dmanoharan",
    "name": "someNamespace",
    "uid": "someuuid",
    "apiVersion": "v1",
    "resourceVersion": "2716553ddd",
    "fieldPath": "spec.containers{xxx}"
  },
        "reason":"someReason",
        "firstTimestamp": "2019-01-01T21:24:55Z",
        "lastTimestamp": "2019-01-02T21:27:55Z",
        "count": 10
    }`

	tables := helper_get_k8Watchtable(keys, t, someEventPayload)
	res, err := GetEventData(values, tables, someTs.Add(-1*time.Hour), someTs.Add(6*time.Hour), someRequestId)
	assert.Nil(t, err)
	expectedRes := `[
 {
  "partitionId": "001546398000",
  "namespace": "someNamespace",
  "name": "someName.xx",
  "watchTimestamp": "2019-01-02T03:04:05.000000006Z",
  "kind": "Event",
  "payload": "{\n  \"involvedObject\": {\n    \"kind\": \"someKinda\",\n    \"namespace\": \"user-dmanoharan\",\n    \"name\": \"someNamespace\",\n    \"uid\": \"someuuid\",\n    \"apiVersion\": \"v1\",\n    \"resourceVersion\": \"2716553ddd\",\n    \"fieldPath\": \"spec.containers{xxx}\"\n  },\n        \"reason\":\"someReason\",\n        \"firstTimestamp\": \"2019-01-01T21:24:55Z\",\n        \"lastTimestamp\": \"2019-01-02T21:27:55Z\",\n        \"count\": 10\n    }",
  "eventKey": "/watch/001546398000/Event/someNamespace/someName.xx/1546398245000000006"
 }
]`
	assertex.JsonEqual(t, expectedRes, string(res))
}

func Test_GetEventData_ValPayloadNotInTimeRange(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()
	values[KindParam] = []string{"someKinda"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	var keys []string
	keys = append(keys, typed.NewWatchTableKey(partitionId, "Event", "someNamespace", "someName.xx", someTs).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, "Event", "someNamespaceb", "someName.xx", someTs).String())
	someEventPayload := `{
  "involvedObject": {
    "kind": "someKinda",
    "namespace": "user-dmanoharan",
    "name": "someNamespace",
    "uid": "someuuid",
    "apiVersion": "v1",
    "resourceVersion": "2716553ddd",
    "fieldPath": "spec.containers{xxx}"
  },
        "reason":"someReason",
        "firstTimestamp": "2001-01-01T21:24:55Z",
        "lastTimestamp": "2001-01-02T21:27:55Z",
        "count": 10
    }`

	tables := helper_get_k8Watchtable(keys, t, someEventPayload)
	res, err := GetEventData(values, tables, someTs.Add(-1*time.Hour), someTs.Add(6*time.Hour), someRequestId)
	assert.Nil(t, err)
	assert.Equal(t, string(res), "")
}

func Test_GetEventData_ValPayloadKindNotMatch(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()
	values[KindParam] = []string{"someKinda"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	var keys []string
	keys = append(keys, typed.NewWatchTableKey(partitionId, "Event", "someNamespace", "someName.xx", someTs).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, "Event", "someNamespaceb", "someName.xx", someTs).String())
	someEventPayload := `{
  "involvedObject": {
    "kind": "testkind",
    "namespace": "user-dmanoharan",
    "name": "someNamespace",
    "uid": "someuuid",
    "apiVersion": "v1",
    "resourceVersion": "2716553ddd",
    "fieldPath": "spec.containers{xxx}"
  },
        "reason":"someReason",
        "firstTimestamp": "2019-01-01T21:24:55Z",
        "lastTimestamp": "2019-01-02T21:27:55Z",
        "count": 10
    }`

	tables := helper_get_k8Watchtable(keys, t, someEventPayload)
	res, err := GetEventData(values, tables, someTs.Add(-1*time.Hour), someTs.Add(6*time.Hour), someRequestId)
	assert.Nil(t, err)
	assert.Equal(t, string(res), "")
}
