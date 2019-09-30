/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/test/assertex"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func helper_getResSum(t *testing.T) *typed.ResourceSummary {
	firstTimeProto, err := ptypes.TimestampProto(someFirstSeenTime)
	assert.Nil(t, err)
	lastTimeProto, err := ptypes.TimestampProto(someLastSeenTime)
	assert.Nil(t, err)
	createTimeProto, err := ptypes.TimestampProto(someCreateTime)
	assert.Nil(t, err)
	val := &typed.ResourceSummary{
		FirstSeen:    firstTimeProto,
		LastSeen:     lastTimeProto,
		CreateTime:   createTimeProto,
		DeletedAtEnd: false,
	}
	return val
}

func Test_GetResSummaryData_False(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	values := helper_get_params()
	values[KindParam] = []string{"someKind"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	values[UuidParam] = []string{"someuid"}
	keys := make([]*typed.ResourceSummaryKey, 2)
	keys[0] = typed.NewResourceSummaryKey(someTs, "someKind", "someNs", "mynamespace", "68510937-4ffc-11e9-8e26-1418775557c8")
	keys[1] = typed.NewResourceSummaryKey(someTs, "SomeKind", "namespace-b", "somename-b", "45510937-d4fc-11e9-8e26-14187754567")
	tables := helper_get_resSumtable(keys, t)
	res, err := GetResSummaryData(values, tables, someTs.Add(-60*time.Minute), someTs.Add(60*time.Minute), someRequestId)
	assert.Equal(t, string(res), "")
	assert.Nil(t, err)
}

func Test_GetResSummaryData_NotInTimeRange(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	values := helper_get_params()
	values[KindParam] = []string{"someKind"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	values[UuidParam] = []string{"someuid"}
	keys := make([]*typed.ResourceSummaryKey, 1)
	keys[0] = typed.NewResourceSummaryKey(someTs, "someKind", "someNamespace", "someName", "someuid")
	tables := helper_get_resSumtable(keys, t)
	res, err := GetResSummaryData(values, tables, someTs.Add(60*time.Minute), someTs.Add(160*time.Minute), someRequestId)
	assert.Nil(t, err)
	assert.Equal(t, string(res), "")
}

func Test_GetResSummaryData_True(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	values := helper_get_params()
	values[KindParam] = []string{"someKind"}
	values[NamespaceParam] = []string{"someNamespace"}
	values[NameParam] = []string{"someName"}
	values[UuidParam] = []string{"someuid"}
	keys := make([]*typed.ResourceSummaryKey, 1)
	keys[0] = typed.NewResourceSummaryKey(someFirstSeenTime, "someKind", "someNamespace", "someName", "someuid")
	tables := helper_get_resSumtable(keys, t)
	res, err := GetResSummaryData(values, tables, someFirstSeenTime.Add(-1*time.Hour), someLastSeenTime.Add(6*time.Hour), someRequestId)
	assert.Nil(t, err)
	expectedRes := `{
       "PartitionId": "001551668400",
       "Kind": "someKind",
       "Namespace": "someNamespace",
       "Name": "someName",
       "Uid": "someuid",
       "firstSeen": {
           "seconds": 1551668645,
          "nanos": 6
       },
       "lastSeen": {
          "seconds": 1551837840
       }
     }`
	assertex.JsonEqual(t, expectedRes, string(res))
}
