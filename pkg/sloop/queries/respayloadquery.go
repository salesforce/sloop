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
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"net/url"
	"time"
)

type ResPayLoadData struct {
	PayLoadMap map[int64]string `json:"payloadMap"`
}

func GetResPayload(params url.Values, t typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error) {
	var watchRes map[typed.WatchTableKey]*typed.KubeWatchResult
	previousKeyFound := false
	var previousKey *typed.WatchTableKey
	var previousVal *typed.KubeWatchResult
	var previousErr error

	selectedNamespace := params.Get(NamespaceParam)
	selectedName := params.Get(NameParam)
	selectedKind := params.Get(KindParam)

	if kubeextractor.IsClustersScopedResource(selectedKind) {
		selectedNamespace = DefaultNamespace
	}

	err := t.Db().View(func(txn badgerwrap.Txn) error {
		var err2 error
		var stats typed.RangeReadStats

		keyComparator := typed.NewWatchTableKeyComparator(selectedKind, selectedNamespace, selectedName, time.Time{})
		valPredFn := typed.KubeWatchResult_ValPredicateFns(isResPayloadInTimeRange(startTime, endTime))
		watchRes, _, err2 = t.WatchTable().RangeRead(txn, keyComparator, nil, valPredFn, startTime, endTime)
		if err2 != nil {
			return err2
		}

		// get the previous key for those who has same payload but just before startTime
		seekKey := keyComparator
		seekKey.PartitionId = untyped.GetPartitionId(startTime)
		previousKey, err2 = t.WatchTable().GetPreviousKey(txn, seekKey, keyComparator)

		// when err2 is not nil, we will not return err since it is ok we did not find previous key from startTime,
		// we can continue using the result from rangeRead to proceed the rest payload
		if err2 == nil {
			previousVal, previousErr = t.WatchTable().Get(txn, previousKey.String())
			if previousErr == nil {
				previousKeyFound = true
			}
		}

		stats.Log(requestId)
		return nil
	})
	if err != nil {
		return []byte{}, err
	}

	var res ResPayLoadData
	resPayloadMap := make(map[int64]string)
	for key, val := range watchRes {
		resPayloadMap[key.Timestamp.Unix()] = val.Payload
	}

	// when previousKey is found, add it to the payload map as well
	if previousKeyFound {
		resPayloadMap[previousKey.Timestamp.Unix()] = previousVal.Payload
	}

	glog.V(5).Infof("get the length of the resPayload is:%v", len(resPayloadMap))
	if len(resPayloadMap) == 0 {
		return []byte{}, nil
	}

	res.PayLoadMap = resPayloadMap
	bytes, err := json.MarshalIndent(res.PayLoadMap, "", " ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json %v", err)
	}
	return bytes, nil
}
