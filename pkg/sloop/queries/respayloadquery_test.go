/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"github.com/dgraph-io/badger"
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
	assert.Equal(t, string(res), "")
	assert.Nil(t, err)
}

func Test_GetResPayload_NotInTimeRange(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()
	values[KindParam] = []string{"someKind"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	var keys []string
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind", "someNamespacea", "someName", someTs).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind", "someNamespaceb", "someName", someTs.Add(-1*time.Hour)).String())
	for i := 'b'; i < 'd'; i++ {
		keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind"+string(i), "someNamespace", "someName.xx", someTs).String())
	}
	tables := helper_get_resPayload(keys, t, somePTime)
	res, err := GetResPayload(values, tables, someTs.Add(2*time.Hour), someTs.Add(5*time.Hour), someRequestId)
	assert.Nil(t, err)
	assert.Equal(t, string(res), "")
}

func Test_GetResPayload_True_NoPreviousKeyFound(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)
	values := helper_get_params()
	values[KindParam] = []string{"someKind"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	var keys []string
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind", "someNamespace", "someName", someTs).String())
	keys = append(keys, typed.NewWatchTableKey(partitionId, "someKind", "someNamespaceb", "someName", someTs).String())
	tables := helper_get_resPayload(keys, t, somePTime)
	res, err := GetResPayload(values, tables, someTs.Add(-1*time.Hour), someTs.Add(6*time.Hour), someRequestId)
	assert.Nil(t, err)
	expectedRes := `{
 "1546398245": "{\n  \"metadata\": {\n    \"name\": \"someName\",\n    \"namespace\": \"someNamespace\",\n    \"uid\": \"6c2a9795-a282-11e9-ba2f-14187761de09\",\n    \"creationTimestamp\": \"2019-07-09T19:47:45Z\"\n  }\n}"
}`
	assertex.JsonEqual(t, expectedRes, string(res))
}
