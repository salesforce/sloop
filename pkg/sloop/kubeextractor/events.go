/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package kubeextractor

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"strings"
	"time"
)

// Example Event
//
// Name: na1-mist61app-prd-676c5b7dd4-h7x6r.15bf81c8df2bce2c
// Kind: Event
// Namespace: somens
// Payload:
/*
{
  "metadata": {
    "name": "na1-mist61app-prd-676c5b7dd4-h7x6r.15bf81c8df2bce2c",
    "namespace": "somens",
    "selfLink": "/api/v1/namespaces/somens/events/na1-mist61app-prd-676c5b7dd4-h7x6r.15bf81c8df2bce2c",
    "uid": "d73fbbd4-caa3-11e9-a836-5e785cdb595d",
    "resourceVersion": "2623487073",
    "creationTimestamp": "2019-08-29T21:27:45Z"
  },
  "involvedObject": {
    "kind": "Pod",
    "namespace": "somens",
    "name": "na1-mist61app-prd-676c5b7dd4-h7x6r",
    "uid": "2358ba5b-caa3-11e9-a863-14187760f413",
    "apiVersion": "v1",
    "resourceVersion": "2621648750",
    "fieldPath": "spec.containers{coreapp}"
  },
  "reason": "Unhealthy",
  "message": "Readiness probe failed for some reason",
  "source": {
    "component": "kubelet",
    "host": "somehostname"
  },
  "firstTimestamp": "2019-08-29T21:24:55Z",
  "lastTimestamp": "2019-08-30T16:47:45Z",
  "count": 13954,
  "type": "Warning"
}
*/

// Extracts involved object from kube watch event payload.
func ExtractInvolvedObject(payload string) (KubeInvolvedObject, error) {
	resource := struct {
		InvolvedObject KubeInvolvedObject
	}{}
	err := json.Unmarshal([]byte(payload), &resource)
	if err != nil {
		return KubeInvolvedObject{}, err
	}
	return resource.InvolvedObject, nil
}

type EventInfo struct {
	Reason         string    `json:"reason"`
	Type           string    `json:"type"`
	FirstTimestamp time.Time `json:"firstTimestamp"`
	LastTimestamp  time.Time `json:"lastTimestamp"`
	Count          int       `json:"count"`
}

// Extracts event reason from kube watch event payload
func ExtractEventInfo(payload string) (*EventInfo, error) {
	internalResource := struct {
		Reason         string `json:"reason"`
		FirstTimestamp string `json:"firstTimestamp"`
		LastTimestamp  string `json:"lastTimestamp"`
		Count          int    `json:"count"`
		Type           string `json:"type"`
	}{}
	err := json.Unmarshal([]byte(payload), &internalResource)
	if err != nil {
		return nil, err
	}
	// Convert timestamps

	fs, err := time.Parse(time.RFC3339, internalResource.FirstTimestamp)
	if err != nil {
		glog.Errorf("Could not parse first timestamp %v\n", internalResource.FirstTimestamp)
		fs = time.Time{}
	}

	ls, err := time.Parse(time.RFC3339, internalResource.LastTimestamp)
	if err != nil {
		glog.Errorf("Could not parse last timestamp %v\n", internalResource.LastTimestamp)
		fs = time.Time{}
	}

	return &EventInfo{
		Reason:         internalResource.Reason,
		FirstTimestamp: fs,
		LastTimestamp:  ls,
		Count:          internalResource.Count,
		Type:           internalResource.Type,
	}, nil
}

// Events in kubernetes share the same namespace as the involved object, Kind=Event, and
// Name is the involved object name + "." + some unique string
//
// Deployment name: some-deployment-name
// Event name:      some-deployment-name.15c37e2c4b7ff38e
//
// Pod name:        some-deployment-name-d72v-5fd4f779f7-h4t6r
// Event name:      some-deployment-name-d72v-5fd4f779f7-h4t6r.15c37e4fcf9f159f
func GetInvolvedObjectNameFromEventName(eventName string) (string, error) {
	dotIdx := strings.LastIndex(eventName, ".")
	if dotIdx < 0 {
		return "", fmt.Errorf("unexpected format for a k8s event name: %v", eventName)
	}
	return eventName[0:dotIdx], nil
}

func IsClustersScopedResource(selectedKind string) bool {
	if selectedKind == NodeKind || selectedKind == NamespaceKind {
		return true
	}
	return false
}
