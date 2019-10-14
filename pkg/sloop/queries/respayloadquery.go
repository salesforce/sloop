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
	"time"
)

type ResPayLoadData struct {
	PayLoadMap map[int64]string `json:"payloadMap"`
}

func GetResPayload(params url.Values, t typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error) {
	var watchRes map[typed.WatchTableKey]*typed.KubeWatchResult
	err := t.Db().View(func(txn badgerwrap.Txn) error {
		var err2 error
		var stats typed.RangeReadStats

		selectedNamespace := params.Get(NamespaceParam)
		selectedName := params.Get(NameParam)
		selectedKind := params.Get(KindParam)

		if kubeextractor.IsClustersScopedResource(selectedKind) {
			selectedNamespace = DefaultNamespace
		}

		key := &typed.WatchTableKey{
			// partition id will be rest, it is ok to leave it as empty string
			PartitionId: "",
			Kind:        selectedKind,
			Namespace:   selectedNamespace,
			Name:        selectedName,
			Timestamp:   time.Time{},
		}

		valPredFn := typed.KubeWatchResult_ValPredicateFns(isResPayloadInTimeRange(startTime, endTime))
		watchRes, _, err2 = t.WatchTable().RangeRead(txn, key, nil, valPredFn, startTime, endTime)
		if err2 != nil {
			return err2
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
	// Todo: in future we might need to think if we want to return a marshalled empty json object, for now we just return []byte{}
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
