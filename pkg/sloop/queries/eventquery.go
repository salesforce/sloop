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
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"net/url"
	"time"
)

type EventsData struct {
	EventsList []EventOutput `json:"eventsList"`
}

type EventOutput struct {
	PartitionId    string                          `json:"partitionId"`
	Namespace      string                          `json:"namespace"`
	Name           string                          `json:"name"`
	WatchTimestamp time.Time                       `json:"watchTimestamp,omitempty"`
	Kind           string                          `json:"kind,omitempty"`
	WatchType      typed.KubeWatchResult_WatchType `json:"watchType,omitempty"`
	Payload        string                          `json:"payload,omitempty"`
	EventKey       string                          `json:"eventKey"`
}

func GetEventData(params url.Values, t typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error) {
	var watchEvents map[typed.WatchTableKey]*typed.KubeWatchResult
	err := t.Db().View(func(txn badgerwrap.Txn) error {
		var err2 error
		var stats typed.RangeReadStats
		// TODO: In addition to isEventValInTimeRange we need to also crack open the payload and check the involvedObject kind (+namespace, name, uuid)
		watchEvents, stats, err2 = t.WatchTable().RangeRead(txn, paramEventDataFn(params), isEventValInTimeRange(startTime, endTime), startTime, endTime)
		if err2 != nil {
			return err2
		}
		stats.Log(requestId)
		return nil
	})
	if err != nil {
		return []byte{}, err
	}
	var res EventsData
	eventsList := []EventOutput{}
	for key, val := range watchEvents {
		output := EventOutput{
			PartitionId:    key.PartitionId,
			Namespace:      key.Namespace,
			Name:           key.Name,
			WatchTimestamp: key.Timestamp,
			Kind:           key.Kind,
			WatchType:      val.WatchType,
			Payload:        val.Payload,
			EventKey:       key.String(),
		}
		eventsList = append(eventsList, output)
	}

	if len(eventsList) == 0 {
		return []byte{}, nil
	}

	res.EventsList = eventsList
	bytes, err := json.MarshalIndent(res.EventsList, "", " ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json %v", err)
	}
	return bytes, nil
}
