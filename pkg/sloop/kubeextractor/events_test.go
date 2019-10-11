/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package kubeextractor

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_ExtractInvolvedObject_OutputCorrect(t *testing.T) {
	payload := `{"involvedObject":{"kind":"ReplicaSet","namespace":"namespace1","name":"name1","uid":"uid1"}}`
	expectedResult := KubeInvolvedObject{
		Kind:      "ReplicaSet",
		Name:      "name1",
		Namespace: "namespace1",
		Uid:       "uid1",
	}
	result, err := ExtractInvolvedObject(payload)
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result)
}

func Test_ExtractInvolvedObject_InvalidPayload_ReturnsError(t *testing.T) {
	payload := `{"involvedObject":{"name":"name1","namespace":"namespace1","selfLink":"link1"}`
	result, err := ExtractInvolvedObject(payload)
	assert.NotNil(t, err)
	assert.Equal(t, KubeInvolvedObject{}, result)
}

func Test_ExtractInvolvedObject_PayloadHasAdditionalFields_OutputCorrect(t *testing.T) {
	payload := `{"metadata":{"name":"name2","namespace":"namespace2","uid":"uid2"},"involvedObject":{"kind":"Pod","name":"name1","namespace":"namespace1","uid":"uid1"}}`
	expectedResult := KubeInvolvedObject{
		Kind:      "Pod",
		Name:      "name1",
		Namespace: "namespace1",
		Uid:       "uid1",
	}
	result, err := ExtractInvolvedObject(payload)
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result)
}

var someFirstSeenTime = time.Date(2019, 8, 29, 21, 24, 55, 0, time.UTC)
var someLastSeenTime = time.Date(2019, 8, 30, 16, 47, 45, 0, time.UTC)

func Test_ExtractEventReason_OutputCorrect(t *testing.T) {
	payload := `{"reason":"failed","firstTimestamp": "2019-08-29T21:24:55Z","lastTimestamp": "2019-08-30T16:47:45Z","count": 13954}`
	result, err := ExtractEventInfo(payload)
	assert.Nil(t, err)
	assert.Equal(t, "failed", result.Reason)
	assert.Equal(t, someFirstSeenTime, result.FirstTimestamp)
	assert.Equal(t, someLastSeenTime, result.LastTimestamp)
	assert.Equal(t, 13954, result.Count)
}

func Test_ExtractEventReason_MissingFieldsAreIgnored(t *testing.T) {
	payload := `{"metadata":{"name":"name1","uid":"uid1","resourceVersion":"123","creationTimestamp":"2019-07-12T20:12:12Z"}}`
	expectedResult := ""
	result, err := ExtractEventInfo(payload)
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result.Reason)
}

func Test_GetInvolvedObjectNameFromEventName_invalid(t *testing.T) {
	eventName := "xxx"
	key, err := GetInvolvedObjectNameFromEventName(eventName)
	assert.NotNil(t, err)
	assert.Equal(t, key, "")
}

func Test_GetInvolvedObjectNameFromEventName_valid(t *testing.T) {
	eventName := "xxx.abc"
	key, err := GetInvolvedObjectNameFromEventName(eventName)
	assert.Nil(t, err)
	assert.Equal(t, "xxx", key)
}

func Test_GetInvolvedObjectNameFromEventName_HostName(t *testing.T) {
	eventName := "somehost.somedomain.com.abc"
	key, err := GetInvolvedObjectNameFromEventName(eventName)
	assert.Nil(t, err)
	assert.Equal(t, "somehost.somedomain.com", key)
}

func Test_IsClustersScopedResource_True(t *testing.T) {
	selectedKind := NodeKind
	res := IsClustersScopedResource(selectedKind)
	assert.True(t, res)

	res = IsClustersScopedResource(NamespaceKind)
	assert.True(t, res)
}

func Test_IsClustersScopedResource_False(t *testing.T) {
	selectedKind := "someKind"
	res := IsClustersScopedResource(selectedKind)
	assert.False(t, res)
}
