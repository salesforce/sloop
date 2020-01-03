/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/salesforce/sloop/pkg/sloop/test/assertex"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
	"time"
)

var (
	someTs            = time.Date(2019, 1, 2, 3, 4, 5, 6, time.UTC)
	somePTime, _      = ptypes.TimestampProto(someTs)
	someFirstSeenTime = time.Date(2019, 3, 4, 3, 4, 5, 6, time.UTC)
	mostRecentTime    = time.Date(2019, 3, 6, 3, 4, 0, 0, time.UTC)
	someLastSeenTime  = mostRecentTime.Add(-1 * time.Hour)
	someCreateTime    = mostRecentTime.Add(-3 * time.Hour)
)

func helper_get_resSumtable(keys []*typed.ResourceSummaryKey, t *testing.T) typed.Tables {
	firstSeen, err := ptypes.TimestampProto(someFirstSeenTime)
	assert.Nil(t, err)
	lastSeen, err := ptypes.TimestampProto(someLastSeenTime)
	assert.Nil(t, err)
	val := &typed.ResourceSummary{FirstSeen: firstSeen, LastSeen: lastSeen}

	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)
	wt := typed.OpenResourceSummaryTable()

	err = db.Update(func(txn badgerwrap.Txn) error {
		for _, key := range keys {
			txerr := wt.Set(txn, key.String(), val)
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

func helper_get_params() url.Values {
	return map[string][]string{
		QueryParam:          []string{"EventHeatMap"},
		NamespaceParam:      []string{"some-namespace"},
		KindParam:           []string{AllNamespaces},
		LookbackParam:       []string{"24h"},
		"dhxr1567107277290": []string{"1"},
	}
}

func Test_GetNamespaces_Success(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	keys := make([]*typed.ResourceSummaryKey, 2)
	keys[0] = typed.NewResourceSummaryKey(someTs, "Namespace", "", "mynamespace", "68510937-4ffc-11e9-8e26-1418775557c8")
	keys[1] = typed.NewResourceSummaryKey(someTs, "Deployment", "namespace-b", "somename-b", "45510937-d4fc-11e9-8e26-14187754567")
	tables := helper_get_resSumtable(keys, t)

	filterData, err := NamespaceQuery(url.Values{}, tables, someTs, someTs, someRequestId)

	assert.Nil(t, err)
	expectedNamespaces := `[
 "mynamespace",
 "_all"
]`
	assertex.JsonEqual(t, expectedNamespaces, string(filterData))
}

func Test_GetNamespaces_EmptyNamespace(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	keys := make([]*typed.ResourceSummaryKey, 2)
	keys[0] = typed.NewResourceSummaryKey(someTs, "SomeKind", "namespace-a", "mynamespace", "68510937-4ffc-11e9-8e26-1418775557c8")
	keys[1] = typed.NewResourceSummaryKey(someTs, "SomeKind", "namespace-b", "somename-b", "45510937-d4fc-11e9-8e26-14187754567")
	tables := helper_get_resSumtable(keys, t)

	filterData, err := NamespaceQuery(url.Values{}, tables, someTs, someTs, someRequestId)

	assert.Nil(t, err)
	expectedNamespaces := `[
 "_all"
]`
	assertex.JsonEqual(t, expectedNamespaces, string(filterData))
}

func Test_GetKinds_SimpleCase(t *testing.T) {
	untyped.TestHookSetPartitionDuration(time.Hour)
	keys := make([]*typed.ResourceSummaryKey, 2)
	keys[0] = typed.NewResourceSummaryKey(someTs, "Namespace", "", "mynamespace", "68510937-4ffc-11e9-8e26-1418775557c8")
	keys[1] = typed.NewResourceSummaryKey(someTs, "Deployment", "namespace-b", "somename-b", "45510937-d4fc-11e9-8e26-14187754567")
	tables := helper_get_resSumtable(keys, t)

	filterData, err := KindQuery(url.Values{}, tables, someTs, someTs, someRequestId)

	assert.Nil(t, err)
	expectedKinds := `[
 "Deployment",
 "Namespace",
 "_all"
]`
	assertex.JsonEqual(t, expectedKinds, string(filterData))
}

func Test_resSumRowsToNamespaceStrings(t *testing.T) {
	resources := make(map[typed.ResourceSummaryKey]*typed.ResourceSummary)
	firstTimeProto, err := ptypes.TimestampProto(someFirstSeenTime)
	assert.Nil(t, err)
	lastTimeProto, err := ptypes.TimestampProto(someLastSeenTime)
	assert.Nil(t, err)
	createTimeProto, err := ptypes.TimestampProto(someCreateTime)
	assert.Nil(t, err)

	resources[typed.ResourceSummaryKey{
		PartitionId: "0",
		Kind:        "Namespace",
		Namespace:   "",
		Name:        "name1",
		Uid:         "uid1",
	}] = &typed.ResourceSummary{
		FirstSeen:    firstTimeProto,
		LastSeen:     lastTimeProto,
		CreateTime:   createTimeProto,
		DeletedAtEnd: false,
	}
	resources[typed.ResourceSummaryKey{
		PartitionId: "1",
		Kind:        "Namespace",
		Namespace:   "",
		Name:        "name2",
		Uid:         "uid2",
	}] = &typed.ResourceSummary{
		FirstSeen:    firstTimeProto,
		LastSeen:     lastTimeProto,
		CreateTime:   createTimeProto,
		DeletedAtEnd: false,
	}

	// add a duplicate namespace
	resources[typed.ResourceSummaryKey{
		PartitionId: "2",
		Kind:        "Namespace",
		Namespace:   "",
		Name:        "name2",
		Uid:         "uid23",
	}] = &typed.ResourceSummary{
		FirstSeen:    firstTimeProto,
		LastSeen:     lastTimeProto,
		CreateTime:   createTimeProto,
		DeletedAtEnd: true,
	}
	expectedData := []string{"name1", "name2"}
	data := resSumRowsToNamespaceStrings(resources)
	assert.Equal(t, expectedData, data)
}

func Test_isNamespace_Namespace(t *testing.T) {
	// test when kind is namespace
	key1 := "/ressum/001567094400/Namespace//some-othernamespace/96b0e282-9744-11e8-9d31-1418775557c8"
	flag := isNamespace(key1)
	assert.True(t, flag)
}

func Test_isNamespace_KindWithNamespace(t *testing.T) {
	// test when kind is not namespace
	key2 := "/ressum/001562961600/Deployment/some-namespace/some-name/f8f372a3-f731-11e8-b3bd-e24c7f08fac6"
	flag := isNamespace(key2)
	assert.False(t, flag)
}

func Test_isNamespace_KindWithoutNamespce(t *testing.T) {
	// test when there is no namespace field
	key3 := "/eventcount/001567022400/Node//somehost/somehost"
	flag := isNamespace(key3)
	assert.False(t, flag)
}

func Test_isKind_Empty(t *testing.T) {
	kindExists := make(map[string]bool)
	key := "/ressum/001567105200/StatefulSet/some-namespace/some-name/52071bcf-64cf-11e9-b4c3-1418774b3e9d"
	flag := isKind(kindExists)(key)
	assert.True(t, flag)
}

func Test_isKind_KindExists(t *testing.T) {
	kindExists := make(map[string]bool)
	kindExists["Deployment"] = true
	key2 := "/ressum/001562961600/Deployment/some-namespace/some-name/f8f372a3-f731-11e8-b3bd-e24c7f08fac6"
	flag := isKind(kindExists)(key2)
	assert.False(t, flag)
}

func Test_resSumRowsToKindStrings(t *testing.T) {
	resources := make(map[typed.ResourceSummaryKey]*typed.ResourceSummary)
	firstTimeProto, err := ptypes.TimestampProto(someFirstSeenTime)
	assert.Nil(t, err)
	lastTimeProto, err := ptypes.TimestampProto(someLastSeenTime)
	assert.Nil(t, err)
	createTimeProto, err := ptypes.TimestampProto(someCreateTime)
	assert.Nil(t, err)

	resources[typed.ResourceSummaryKey{
		PartitionId: "0",
		Kind:        "Pod",
		Namespace:   "",
		Name:        "name1",
		Uid:         "uid1",
	}] = &typed.ResourceSummary{
		FirstSeen:    firstTimeProto,
		LastSeen:     lastTimeProto,
		CreateTime:   createTimeProto,
		DeletedAtEnd: false,
	}
	resources[typed.ResourceSummaryKey{
		PartitionId: "1",
		Kind:        "Deployment",
		Namespace:   "",
		Name:        "name2",
		Uid:         "uid2",
	}] = &typed.ResourceSummary{
		FirstSeen:    firstTimeProto,
		LastSeen:     lastTimeProto,
		CreateTime:   createTimeProto,
		DeletedAtEnd: false,
	}

	// add a duplicate kind
	resources[typed.ResourceSummaryKey{
		PartitionId: "2",
		Kind:        "Deployment",
		Namespace:   "",
		Name:        "name2",
		Uid:         "uid23",
	}] = &typed.ResourceSummary{
		FirstSeen:    firstTimeProto,
		LastSeen:     lastTimeProto,
		CreateTime:   createTimeProto,
		DeletedAtEnd: true,
	}
	expectedData := []string{"", "Deployment", "Pod"}
	data := resSumRowsToKindStrings(resources)
	assert.Equal(t, expectedData, data)
}
