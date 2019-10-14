/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"net/url"
	"strings"
	"time"
)

func paramFilterResSumFn(params url.Values) func(string) bool {
	selectedNamespace := params.Get(NamespaceParam)
	selectedKind := params.Get(KindParam)
	selectedNameSubstring := params.Get(NameMatchParam)
	selectedNameExactMatch := params.Get(NameParam)
	selectedUuid := params.Get(UuidParam)
	return func(key string) bool {
		k := &typed.ResourceSummaryKey{}
		err := k.Parse(key)
		if err != nil {
			return false
		}
		kind := k.Kind
		namespace := k.Namespace
		name := k.Name
		uuid := k.Uid
		return keepRowHelper(name, kind, namespace, selectedKind, selectedNamespace, selectedNameSubstring, selectedNameExactMatch, selectedUuid, uuid)
	}
}

func paramEventCountSumFn(params url.Values) func(string) bool {
	selectedNamespace := params.Get(NamespaceParam)
	selectedKind := params.Get(KindParam)
	selectedNameMatchSubstring := params.Get(NameMatchParam)
	return func(key string) bool {
		k := &typed.EventCountKey{}
		err := k.Parse(key)
		if err != nil {
			return false
		}
		kind := k.Kind
		namespace := k.Namespace
		name := k.Name
		return keepRowHelper(name, kind, namespace, selectedKind, selectedNamespace, selectedNameMatchSubstring, "", "", "")
	}
}

func paramFilterWatchActivityFn(params url.Values) func(string) bool {
	selectedNamespace := params.Get(NamespaceParam)
	selectedKind := params.Get(KindParam)
	selectedNameSubstring := params.Get(NameMatchParam)
	selectedNameExactMatch := params.Get(NameParam)
	selectedUuid := params.Get(UuidParam)
	return func(key string) bool {
		k := &typed.WatchActivityKey{}
		err := k.Parse(key)
		if err != nil {
			return false
		}
		kind := k.Kind
		namespace := k.Namespace
		name := k.Name
		uuid := k.Uid
		return keepRowHelper(name, kind, namespace, selectedKind, selectedNamespace, selectedNameSubstring, selectedNameExactMatch, selectedUuid, uuid)
	}
}

// TODO: Try and remove some of this special logic.  Maybe have a generic approach for resources that dont have namespaces
func keepRowHelper(name string, kind string, namespace string, selectedKind string, selectedNamespace string, selectedNameMatchSubstring string, selectedNameExactMatch string, selectedUuid string, uuid string) bool {
	// Edge cases:
	// 1) Node does not have a namespace
	// 2) Namespace does not have a namespace

	if selectedKind != AllKinds {
		if selectedKind != kind {
			return false
		}
	} else {
		// When showing all kinds and a namespace is set dont show nodes
		if selectedNamespace != AllNamespaces && kind == kubeextractor.NodeKind {
			return false
		}
	}

	// Nodes do not have a namespace.  If user set kind=Node then no need to filter on namespace
	// which would just confuse the user when they dont see the nodes
	if selectedNamespace != AllNamespaces && selectedKind != kubeextractor.NodeKind {
		if kind == kubeextractor.NamespaceKind {
			// A namespace itself does not have a namespace, so instead match on name
			if selectedNamespace != name {
				return false
			}
		} else {
			if selectedNamespace != namespace {
				return false
			}
		}
	}

	if selectedNameMatchSubstring != "" {
		if !strings.Contains(name, selectedNameMatchSubstring) {
			return false
		}
	}

	if selectedNameExactMatch != "" {
		if !strings.EqualFold(name, selectedNameExactMatch) {
			return false
		}
	}

	if selectedUuid != "" {
		if selectedUuid != uuid {
			return false
		}
	}

	return true
}

func isResSummaryValInTimeRange(startTime time.Time, endTime time.Time) func(*typed.ResourceSummary) bool {
	return func(retVal *typed.ResourceSummary) bool {
		firstSeen, err := ptypes.Timestamp(retVal.FirstSeen)
		if err != nil {
			return false
		}

		lastSeen, err := ptypes.Timestamp(retVal.LastSeen)
		if err != nil {
			return false
		}
		if firstSeen.After(endTime) || lastSeen.Before(startTime) {
			return false
		}
		return true
	}
}

func isEventValInTimeRange(startTime time.Time, endTime time.Time) func(*typed.KubeWatchResult) bool {
	return func(retVal *typed.KubeWatchResult) bool {
		eventInfo, err := kubeextractor.ExtractEventInfo(retVal.Payload)
		if err != nil {
			return false
		}
		firstTime := eventInfo.FirstTimestamp
		lastTime := eventInfo.LastTimestamp
		if firstTime.After(endTime) || lastTime.Before(startTime) {
			return false
		}
		return true
	}
}

func matchEventInvolvedObject(params url.Values) func(*typed.KubeWatchResult) bool {
	selectedKind := params.Get(KindParam)
	return func(retVal *typed.KubeWatchResult) bool {
		involvedObj, err := kubeextractor.ExtractInvolvedObject(retVal.Payload)
		if err != nil {
			return false
		}
		if involvedObj.Kind != selectedKind {
			return false
		}
		return true
	}
}

func isResPayloadInTimeRange(startTime time.Time, endTime time.Time) func(*typed.KubeWatchResult) bool {
	return func(retVal *typed.KubeWatchResult) bool {
		resTime, err := ptypes.Timestamp(retVal.Timestamp)
		if err != nil {
			return false
		}
		if resTime.After(endTime) || resTime.Before(startTime) {
			return false
		}
		return true
	}
}
