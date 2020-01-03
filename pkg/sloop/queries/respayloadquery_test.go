/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/salesforce/sloop/pkg/sloop/test/assertex"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const somePodPayload = `{
  "metadata": {
    "name": "someName",
    "namespace": "someNamespace",
    "uid": "6c2a9795-a282-11e9-ba2f-14187761de09",
    "creationTimestamp": "2019-07-09T19:47:45Z"
  }
}`

func helper_get_resPayload(keys []string, t *testing.T, somePTime *timestamp.Timestamp) typed.Tables {
	val := &typed.KubeWatchResult{Kind: "someKind", Timestamp: somePTime, Payload: somePodPayload}
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

func Test_GetResPayload_False(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()
	values[KindParam] = []string{"someKind-Test"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	var keys []string
	for i := 'a'; i < 'd'; i++ {
		keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind"+string(i), "someNamespace", "someName", someTs).String())
	}
	starTime := someTs.Add(-60 * time.Minute)
	endTime := someTs.Add(60 * time.Minute)
	tables := helper_get_resPayload(keys, t, somePTime)
	res, err := GetResPayload(values, tables, starTime, endTime, someRequestId)
	assert.Equal(t, "[]", string(res))
	assert.Nil(t, err)
}

func Test_GetResPayload_NotInTimeRange(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()

	expectedKind := "someKind"
	expectedNS := "someNamespace"
	expectedName := "someName"
	values[KindParam] = []string{expectedKind}
	values[NamespaceParam] = []string{expectedNS}
	values[NameParam] = []string{expectedName}

	var keys []string
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind", "someNamespace-a", "someName", someTs).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind", "someNamespace-b", "someName", someTs.Add(-1*time.Hour)).String())
	for i := 'b'; i < 'd'; i++ {
		keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind"+string(i), "someNamespace", "someName.xx", someTs).String())
	}
	tables := helper_get_resPayload(keys, t, somePTime)
	res, err := GetResPayload(values, tables, someTs.Add(2*time.Hour), someTs.Add(5*time.Hour), someRequestId)
	assert.Nil(t, err)
	assert.Equal(t, "[]", string(res))
}

func Test_GetResPayload_True_NoPreviousKeyFound(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()
	expectedKind := "someKind"
	expectedNS := "someNamespace"
	expectedName := "someName"
	values[KindParam] = []string{expectedKind}
	values[NamespaceParam] = []string{expectedNS}
	values[NameParam] = []string{expectedName}

	var keys []string
	keys = append(keys, typed.NewWatchTableKey(partitionId, expectedKind, expectedNS, expectedName, someTs).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind", "someNamespaceb", "someName", someTs).String())
	tables := helper_get_resPayload(keys, t, somePTime)
	res, err := GetResPayload(values, tables, someTs.Add(-1*time.Hour), someTs.Add(6*time.Hour), someRequestId)
	assert.Nil(t, err)
	expectedRes := `[
 {
  "payloadTime": 1546398245000000006,
  "payload": "{\n  \"metadata\": {\n    \"name\": \"someName\",\n    \"namespace\": \"someNamespace\",\n    \"uid\": \"6c2a9795-a282-11e9-ba2f-14187761de09\",\n    \"creationTimestamp\": \"2019-07-09T19:47:45Z\"\n  }\n}",
  "payloadKey": "/watch/001546398000/someKind/someNamespace/someName/1546398245000000006"
 }
]`
	assertex.JsonEqual(t, expectedRes, string(res))
}

func Test_GetResPayload_True_PreviousKeyFound(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	prevPartitionId := untyped.GetPartitionId(someTs.Add(-1 * time.Hour))

	values := helper_get_params()

	expectedKind := "someKind"
	expectedNS := "someNamespace"
	expectedName := "someName"
	values[KindParam] = []string{expectedKind}
	values[NamespaceParam] = []string{expectedNS}
	values[NameParam] = []string{expectedName}

	var keys []string
	keys = append(keys, typed.NewWatchTableKey(prevPartitionId, expectedKind, expectedNS, expectedName, someTs).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, expectedKind, expectedNS, expectedName, someTs.Add(time.Second)).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind-test", "someNamespace-test", "someName-test", someTs).String())
	tables := helper_get_resPayload(keys, t, somePTime)

	res, err := GetResPayload(values, tables, someTs, someTs.Add(6*time.Hour), someRequestId)

	assert.Nil(t, err)
	expectedRes := `[
 {
  "payloadKey": "/watch/001546394400/someKind/someNamespace/someName/1546398245000000006",
  "payloadTime": 1546398245000000006,
  "payload": "{\n  \"metadata\": {\n    \"name\": \"someName\",\n    \"namespace\": \"someNamespace\",\n    \"uid\": \"6c2a9795-a282-11e9-ba2f-14187761de09\",\n    \"creationTimestamp\": \"2019-07-09T19:47:45Z\"\n  }\n}"
 }
]`
	assertex.JsonEqual(t, expectedRes, string(res))
}

var somePayloadTs = time.Date(2019, 3, 1, 3, 4, 0, 0, time.UTC)

func Test_removeDupePayloads_emptyWorks(t *testing.T) {
	ret := removeDupePayloads([]PayloadOuput{})
	assert.Equal(t, []PayloadOuput{}, ret)
}

func Test_removeDupePayloads_twoUnique_returnsTwo(t *testing.T) {
	input := []PayloadOuput{
		{PayLoadTime: somePayloadTs.Add(time.Minute).UnixNano(), Payload: "abc"},
		{PayLoadTime: somePayloadTs.UnixNano(), Payload: "def"},
	}
	expected := []PayloadOuput{
		{PayLoadTime: somePayloadTs.UnixNano(), Payload: "def"},
		{PayLoadTime: somePayloadTs.Add(time.Minute).UnixNano(), Payload: "abc"},
	}
	ret := removeDupePayloads(input)
	assert.Equal(t, expected, ret)
}

func Test_removeDupePayloads_twoTheSame_returnsFirstOne(t *testing.T) {
	input := []PayloadOuput{
		{PayLoadTime: somePayloadTs.Add(time.Minute).UnixNano(), Payload: "abc"},
		{PayLoadTime: somePayloadTs.UnixNano(), Payload: "abc"},
	}
	expected := []PayloadOuput{
		{PayLoadTime: somePayloadTs.UnixNano(), Payload: "abc"},
	}
	ret := removeDupePayloads(input)
	assert.Equal(t, expected, ret)
}
