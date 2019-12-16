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
	"github.com/dgraph-io/badger/v2"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"net/url"
	"sort"
	"time"
)

type ResPayLoadData struct {
	PayloadList []PayloadOuput `json:"payloadList"`
}

type PayloadOuput struct {
	PayloadKey  string `json:"payloadKey"`
	PayLoadTime int64  `json:"payloadTime"`
	Payload     string `json:"payload,omitempty"`
}

func GetResPayload(params url.Values, t typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error) {
	var watchRes map[typed.WatchTableKey]*typed.KubeWatchResult
	var previousKey *typed.WatchTableKey
	var previousVal *typed.KubeWatchResult

	err := t.Db().View(func(txn badgerwrap.Txn) error {
		var stats typed.RangeReadStats

		keyComparator := getKeyComparator(params)
		valPredFn := typed.KubeWatchResult_ValPredicateFns(isResPayloadInTimeRange(startTime, endTime))

		var rangeReadErr error
		watchRes, _, rangeReadErr = t.WatchTable().RangeRead(txn, keyComparator, nil, valPredFn, startTime, endTime)
		if rangeReadErr != nil {
			return rangeReadErr
		}

		// get the previous key for those who has same payload but just before startTime
		var getPreviousErr error
		seekKey := getSeekKey(keyComparator, startTime)
		previousKey, getPreviousErr = t.WatchTable().GetPreviousKey(txn, seekKey, keyComparator)

		// when getPreviousErr is not nil, we will not return err since it is ok we did not find previous key from startTime,
		// we can continue using the result from rangeRead to proceed the rest payload
		if getPreviousErr == nil {
			var getErr error
			previousVal, getErr = t.WatchTable().Get(txn, previousKey.String())
			if getErr == nil {
				watchRes[*previousKey] = previousVal
			} else {
				// we need to return error when getErr is not nil and its error is not keyNotFound
				if getErr != badger.ErrKeyNotFound {
					return getErr
				}
			}
		}

		stats.Log(requestId)
		return nil
	})
	if err != nil {
		return []byte{}, err
	}

	payloadOutputList := getPayloadOutputList(watchRes)
	glog.V(5).Infof("get the length of the resPayload is:%v", len(payloadOutputList))

	// Sort by time and remove entries with no payload change
	payloadOutputList = removeDupePayloads(payloadOutputList)

	var res ResPayLoadData
	res.PayloadList = payloadOutputList
	bytes, err := json.MarshalIndent(res.PayloadList, "", " ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json for PayloadList  %v", err)
	}

	return bytes, nil
}

//todo: add unit tests
func getSeekKey(keyComparator *typed.WatchTableKey, startTime time.Time) *typed.WatchTableKey {
	seekKey := keyComparator
	seekKey.PartitionId = untyped.GetPartitionId(startTime)
	seekKey.Timestamp = startTime
	return seekKey
}

//todo: add unit tests
func getKeyComparator(params url.Values) *typed.WatchTableKey {
	selectedNamespace := params.Get(NamespaceParam)
	selectedName := params.Get(NameParam)
	selectedKind := params.Get(KindParam)
	if kubeextractor.IsClustersScopedResource(selectedKind) {
		selectedNamespace = ""
	}
	return typed.NewWatchTableKeyComparator(selectedKind, selectedNamespace, selectedName, time.Time{})
}

//todo: add unit tests
func getPayloadOutputList(watchRes map[typed.WatchTableKey]*typed.KubeWatchResult) []PayloadOuput {

	payloadOutputList := []PayloadOuput{}
	for key, val := range watchRes {
		output := PayloadOuput{
			PayLoadTime: key.Timestamp.UnixNano(),
			Payload:     val.Payload,
			PayloadKey:  key.String(),
		}
		payloadOutputList = append(payloadOutputList, output)
	}

	return payloadOutputList
}

func removeDupePayloads(payloads []PayloadOuput) []PayloadOuput {
	sort.Slice(payloads, func(i, j int) bool {
		return payloads[i].PayLoadTime < payloads[j].PayLoadTime
	})

	ret := []PayloadOuput{}

	lastPayload := ""
	for _, val := range payloads {
		if val.Payload != lastPayload {
			ret = append(ret, val)
		}
		lastPayload = val.Payload
	}

	return ret
}
