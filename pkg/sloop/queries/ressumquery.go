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
	"reflect"
	"time"
)

type ResSummaryOutput struct {
	typed.ResourceSummaryKey
	typed.ResourceSummary
}

func (r ResSummaryOutput) IsEmpty() bool {
	return reflect.DeepEqual(ResSummaryOutput{}, r)
}

func GetResSummaryData(params url.Values, t typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error) {
	var resSummaries map[typed.ResourceSummaryKey]*typed.ResourceSummary
	err := t.Db().View(func(txn badgerwrap.Txn) error {
		var err2 error
		var stats typed.RangeReadStats
		resSummaries, stats, err2 = t.ResourceSummaryTable().RangeRead(txn, nil, paramFilterResSumFn(params), isResSummaryValInTimeRange(startTime, endTime), startTime, endTime)
		if err2 != nil {
			return err2
		}
		stats.Log(requestId)
		return nil
	})
	if err != nil {
		return []byte{}, err
	}

	output := ResSummaryOutput{}
	for key, val := range resSummaries {
		output.PartitionId = key.PartitionId
		output.Name = key.Name
		output.Namespace = key.Namespace
		output.Uid = key.Uid
		output.Kind = key.Kind
		output.FirstSeen = val.FirstSeen
		output.LastSeen = val.LastSeen
		output.CreateTime = val.CreateTime
		output.DeletedAtEnd = val.DeletedAtEnd
		output.Relationships = val.Relationships

		// we only need to get one resSummary
		break
	}

	if output.IsEmpty() {
		return []byte{}, nil
	}

	bytes, err := json.MarshalIndent(output, "", " ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json %v", err)
	}
	return bytes, nil
}
