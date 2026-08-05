/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"

	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

const somePodPayloadWithContainers = `{
  "metadata": {
    "name": "somePod",
    "namespace": "someNamespace",
    "uid": "6c2a9795-a282-11e9-ba2f-14187761de09",
    "creationTimestamp": "2019-01-02T03:04:05Z",
    "labels": {"app": "someApp"}
  },
  "spec": {
    "nodeName": "someNode",
    "containers": [
      {
        "name": "database",
        "image": "gcr.io/some-registry/someimage:15",
        "ports": [{"containerPort": 5432, "protocol": "TCP"}]
      }
    ]
  },
  "status": {
    "phase": "Running",
    "containerStatuses": [
      {
        "name": "database",
        "image": "gcr.io/some-registry/someimage:15",
        "imageID": "gcr.io/some-registry/someimage@sha256:4f2e646a914a3c441ef2be6a1d55a14e2c006a5eb1c6a2569cf6649f57b02549",
        "containerID": "containerd://c133b3454d0aa5ffe846f218b63f281cbc00c7125ea86b375cd1acb1eedb2e25",
        "restartCount": 0,
        "ready": true,
        "state": {"running": {"startedAt": "2019-01-02T03:04:10Z"}}
      }
    ]
  }
}`

const someEventPayload = `{
  "metadata": {
    "name": "somePod.157de23babbb985d",
    "namespace": "someNamespace",
    "uid": "0a4selfb-a282-11e9-ba2f-14187761de09"
  },
  "involvedObject": {"kind": "Pod", "name": "somePod", "namespace": "someNamespace"},
  "reason": "Started",
  "message": "Started container database",
  "type": "Normal",
  "count": 1,
  "source": {"component": "kubelet", "host": "someNode"},
  "firstTimestamp": "2019-01-02T03:04:10Z",
  "lastTimestamp": "2019-01-02T03:04:10Z"
}`

const someCrdPayload = `{
  "apiVersion": "example.com/v1",
  "kind": "SomeCrd",
  "metadata": {
    "name": "someCrd",
    "namespace": "someNamespace",
    "labels": {"team": "someTeam"}
  },
  "spec": {"replicaCount": 3, "apiEndpoint": "https://example.com"}
}`

func helper_get_describe_tables(t *testing.T, records map[string]string) typed.Tables {
	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wt := typed.OpenKubeWatchResultTable()

	err = db.Update(func(txn badgerwrap.Txn) error {
		for key, payload := range records {
			val := &typed.KubeWatchResult{Kind: "Pod", Timestamp: somePTime, Payload: payload}
			txerr := wt.Set(txn, key, val)
			if txerr != nil {
				return txerr
			}
		}
		return nil
	})
	assert.Nil(t, err)
	return typed.NewTableList(db)
}

func helper_run_describe(t *testing.T, kind string, name string, records map[string]string) []DescribeOutput {
	values := helper_get_params()
	values[KindParam] = []string{kind}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{name}

	tables := helper_get_describe_tables(t, records)
	res, err := GetResDescribe(values, tables, someTs.Add(-1*time.Hour), someTs.Add(time.Hour), someRequestId)
	assert.Nil(t, err)

	var describeList []DescribeOutput
	assert.Nil(t, json.Unmarshal(res, &describeList))
	return describeList
}

func Test_GetResDescribe_PodShowsImageIdAndEvents(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)

	records := map[string]string{
		typed.NewWatchTableKey(partitionId, "Pod", "someNamespace", "somePod", someTs).String():                    somePodPayloadWithContainers,
		typed.NewWatchTableKey(partitionId, "Event", "someNamespace", "somePod.157de23babbb985d", someTs).String(): someEventPayload,
		typed.NewWatchTableKey(partitionId, "Pod", "someNamespace", "someOtherPod", someTs).String():               somePodPayload,
	}

	describeList := helper_run_describe(t, "Pod", "somePod", records)
	assert.Equal(t, 1, len(describeList))

	describeText := describeList[0].Describe
	assert.Regexp(t, `Name:\s+somePod`, describeText)
	assert.Contains(t, describeText, "Image ID:")
	assert.Contains(t, describeText, "sha256:4f2e646a914a3c441ef2be6a1d55a14e2c006a5eb1c6a2569cf6649f57b02549")
	assert.Contains(t, describeText, "Container ID:")
	assert.Contains(t, describeText, "Events:")
	assert.Contains(t, describeText, "Started container database")
}

func Test_GetResDescribe_OnePerDistinctPayload(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)

	changedPod := somePodPayloadWithContainers[:len(somePodPayloadWithContainers)-1] + `,"spec2": {"x": 1}}`
	records := map[string]string{
		typed.NewWatchTableKey(partitionId, "Pod", "someNamespace", "somePod", someTs).String():                    somePodPayloadWithContainers,
		typed.NewWatchTableKey(partitionId, "Pod", "someNamespace", "somePod", someTs.Add(time.Minute)).String():   somePodPayloadWithContainers,
		typed.NewWatchTableKey(partitionId, "Pod", "someNamespace", "somePod", someTs.Add(2*time.Minute)).String(): changedPod,
	}

	// the unchanged payload collapses; the changed one does not parse differently for describe
	// (unknown field), so it also collapses into the first describe output
	describeList := helper_run_describe(t, "Pod", "somePod", records)
	assert.Equal(t, 1, len(describeList))
}

func Test_GetResDescribe_UnknownKindUsesGenericDescribe(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	partitionId := untyped.GetPartitionId(someTs)

	records := map[string]string{
		typed.NewWatchTableKey(partitionId, "SomeCrd", "someNamespace", "someCrd", someTs).String(): someCrdPayload,
	}

	describeList := helper_run_describe(t, "SomeCrd", "someCrd", records)
	assert.Equal(t, 1, len(describeList))

	describeText := describeList[0].Describe
	assert.Regexp(t, `Name:\s+someCrd`, describeText)
	assert.Regexp(t, `Namespace:\s+someNamespace`, describeText)
	assert.Contains(t, describeText, "team=someTeam")
	assert.Contains(t, describeText, "API Version:")
	assert.Regexp(t, `Replica Count:\s+3`, describeText)
	assert.Contains(t, describeText, "API Endpoint:")
}

func Test_smartLabelFor(t *testing.T) {
	assert.Equal(t, "Container Statuses", smartLabelFor("containerStatuses"))
	assert.Equal(t, "API Version", smartLabelFor("apiVersion"))
	assert.Equal(t, "UID", smartLabelFor("uid"))
	assert.Equal(t, "Host IP", smartLabelFor("hostIP"))
	assert.Equal(t, "some.field/name", smartLabelFor("some.field/name"))
}
