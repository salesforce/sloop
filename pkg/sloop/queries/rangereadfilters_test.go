/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_isFiltered_NotSelectedNamespace(t *testing.T) {
	values := helper_get_params()

	// test when namespace is not selected
	key := "/eventcount/001567105200/StatefulSet/some-user/vrb-mgmt-pd/52071bcf-64cf-11e9-b4c3-1418774b3e9d"
	flag := paramEventCountSumFn(values)(key)
	assert.False(t, flag)
}

func Test_isFiltered_SelectedNamespace(t *testing.T) {
	values := helper_get_params()
	// test when namespace is selected
	key := "/eventcount/001567105200/StatefulSet/some-namespace/vrb-mgmt-pd/52071bcf-64cf-11e9-b4c3-1418774b3e9d"
	flag := paramEventCountSumFn(values)(key)
	assert.True(t, flag)
}

func Test_isFiltered_WhenKindIsNodeIgnoreNamespace(t *testing.T) {
	values := helper_get_params()
	values["kind"] = []string{kubeextractor.NodeKind}
	values["namespace"] = []string{"someNamespace"}
	// test node
	key := "/eventcount/001567022400/Node//somehost/somehost"
	flag := paramEventCountSumFn(values)(key)
	assert.True(t, flag)
}

func Test_isFiltered_NodeReturnedForAllNamespaces(t *testing.T) {
	values := helper_get_params()
	values["kind"] = []string{kubeextractor.NodeKind}
	values["namespace"] = []string{AllNamespaces}
	// test node
	key := "/eventcount/001567022400/Node//somehost/somehost"
	flag := paramEventCountSumFn(values)(key)
	assert.True(t, flag)
}

func Test_isFiltered_NodeNotReturnedForAllKindsInSomeNamespace(t *testing.T) {
	values := make(map[string][]string)
	values["kind"] = []string{AllKinds}
	values["namespace"] = []string{"foo"}
	// test node
	key := "/eventcount/001567022400/Node//somehost/somehost"
	flag := paramEventCountSumFn(values)(key)
	assert.False(t, flag)
}

func Test_isFiltered_NodeReturnedForAllKindsAllNamespace(t *testing.T) {
	values := make(map[string][]string)
	values["kind"] = []string{AllKinds}
	values["namespace"] = []string{AllNamespaces}
	// test node
	key := "/eventcount/001567022400/Node//somehost/somehost"
	flag := paramEventCountSumFn(values)(key)
	assert.True(t, flag)
}

func Test_isFiltered_KindIsNamespaceMatchNameNotNamespace(t *testing.T) {
	values := make(map[string][]string)
	values[NamespaceParam] = []string{"some-namespace"}
	values[KindParam] = []string{kubeextractor.NamespaceKind}
	// test when namespace is not selected
	key := "/ressum/001567094400/Namespace//some-othernamespace/96b0e282-9744-11e8-9d31-1418775557c8"
	flag := paramFilterResSumFn(values)(key)
	assert.False(t, flag)

	// test when namespace is selected
	key = "/ressum/001567094400/Namespace//some-namespace/96b0e282-9744-11e8-9d31-1418775557c8"
	flag = paramFilterResSumFn(values)(key)
	assert.True(t, flag)
}

func Test_isResSummaryValInTimeRange_False(t *testing.T) {
	val := helper_getResSum(t)
	flag := isResSummaryValInTimeRange(someTs.Add(-60*time.Minute), someTs.Add(60*time.Minute))(val)
	assert.False(t, flag)
}

func Test_isResSummaryValInTimeRange_True(t *testing.T) {
	val := helper_getResSum(t)
	flag := isResSummaryValInTimeRange(someFirstSeenTime.Add(-24*time.Hour), someLastSeenTime.Add(24*time.Hour))(val)
	assert.True(t, flag)
}

func Test_isEventValInTimeRange_False(t *testing.T) {
	someEventPayload := `{
        "reason":"someReason",
        "firstTimestamp": "2016-01-01T21:24:55Z",
        "lastTimestamp": "2016-01-02T21:27:55Z",
        "count": 10
    }`
	val := &typed.KubeWatchResult{Kind: "someKind", Payload: someEventPayload}
	flag := isEventValInTimeRange(someTs.Add(-60*time.Minute), someTs.Add(60*time.Minute))(val)
	assert.False(t, flag)
}

func Test_isEventValInTimeRange_True(t *testing.T) {
	someEventPayload := `{
        "reason":"someReason",
        "firstTimestamp": "2019-01-01T21:24:55Z",
        "lastTimestamp": "2019-01-02T21:27:55Z",
        "count": 10
    }`
	val := &typed.KubeWatchResult{Kind: "someKind", Payload: someEventPayload}
	flag := isEventValInTimeRange(someTs.Add(-60*time.Minute), someTs.Add(60*time.Minute))(val)
	assert.True(t, flag)
}

func Test_isResPayloadInTimeRange_True(t *testing.T) {
	val := &typed.KubeWatchResult{Kind: "someKind", Timestamp: somePTime}
	flag := isResPayloadInTimeRange(someTs.Add(-60*time.Minute), someTs.Add(60*time.Minute))(val)
	assert.True(t, flag)
}

func Test_isResPayloadInTimeRange_False(t *testing.T) {
	val := &typed.KubeWatchResult{Kind: "someKind", Timestamp: somePTime}
	flag := isResPayloadInTimeRange(someTs.Add(60*time.Minute), someTs.Add(65*time.Minute))(val)
	assert.False(t, flag)
}
