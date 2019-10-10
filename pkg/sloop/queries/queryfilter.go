/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"encoding/json"
	"fmt"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"net/url"
	"sort"
	"time"
)

// Consider: Make use of resources to limit what namespaces we return.
// For example, if kind == ConfigMap, only return namespaces that contain a ConfigMap
func NamespaceQuery(params url.Values, tables typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error) {
	var resourcesNs map[typed.ResourceSummaryKey]*typed.ResourceSummary
	err := tables.Db().View(func(txn badgerwrap.Txn) error {
		var err2 error
		var stats typed.RangeReadStats
		resourcesNs, stats, err2 = tables.ResourceSummaryTable().RangeRead(txn, nil, isNamespace, nil, startTime, endTime)
		if err2 != nil {
			return err2
		}
		stats.Log(requestId)
		return nil
	})
	if err != nil {
		return []byte{}, err
	}
	namespaces := resSumRowsToNamespaceStrings(resourcesNs)
	namespaces = append(namespaces, AllNamespaces)
	bytes, err := json.MarshalIndent(namespaces, "", " ")
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal json %v", err)
	}
	return bytes, nil
}

// TODO: Only return kinds for the specified namespace
func KindQuery(params url.Values, tables typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error) {
	kindExists := make(map[string]bool)
	err := tables.Db().View(func(txn badgerwrap.Txn) error {
		_, stats, err2 := tables.ResourceSummaryTable().RangeRead(txn, nil, isKind(kindExists), nil, startTime, endTime)
		if err2 != nil {
			return err2
		}
		stats.Log(requestId)
		return nil
	})
	if err != nil {
		return []byte{}, err
	}
	kinds := []string{AllKinds}
	for k, _ := range kindExists {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	bytes, err := json.MarshalIndent(kinds, "", " ")
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal json %v", err)
	}
	return bytes, nil
}

func QueryAvailableQueries(params url.Values, tables typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error) {
	queries := GetNamesOfQueries()
	bytes, err := json.MarshalIndent(queries, "", " ")
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal json %v", err)
	}
	return bytes, nil
}

func resSumRowsToNamespaceStrings(resources map[typed.ResourceSummaryKey]*typed.ResourceSummary) []string {
	namespaceList := []string{}
	namespaceExists := make(map[string]bool)
	for key, _ := range resources {
		_, ok := namespaceExists[key.Name]
		if !ok {
			namespaceList = append(namespaceList, key.Name)
			namespaceExists[key.Name] = true
		}
	}
	sort.Strings(namespaceList)
	return namespaceList
}

func isNamespace(k string) bool {
	key := &typed.ResourceSummaryKey{}
	err := key.Parse(k)
	if err != nil {
		return false
	}
	return key.Kind == kubeextractor.NamespaceKind
}

func isKind(kindExists map[string]bool) func(string) bool {
	return func(key string) bool {
		return keepResourceSummaryKind(key, kindExists)
	}
}

func resSumRowsToKindStrings(resources map[typed.ResourceSummaryKey]*typed.ResourceSummary) []string {
	kindList := []string{""}
	KindExists := make(map[string]bool)
	for key, _ := range resources {
		if _, ok := KindExists[key.Kind]; !ok {
			kindList = append(kindList, key.Kind)
			KindExists[key.Kind] = true
		}
	}
	sort.Strings(kindList)
	return kindList
}

func keepResourceSummaryKind(key string, kindExists map[string]bool) bool {
	// parse the key and get its kind
	k := &typed.ResourceSummaryKey{}
	err := k.Parse(key)
	if err != nil {
		return false
	}
	kind := k.Kind

	// if it is the first time to see the kind, return true,
	_, ok := kindExists[kind]
	if !ok {
		kindExists[kind] = true
		return true
	}
	return false
}
