/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"net/url"
	"time"
)

// Takes in arguments from the web page, runs the query, and returns json
type ganttJsonQuery = func(params url.Values, tables typed.Tables, startTime time.Time, endTime time.Time, requestId string) ([]byte, error)

var funcMap = map[string]ganttJsonQuery{
	"EventHeatMap":      EventHeatMap3Query,
	"GetEventData":      GetEventData,
	"GetResPayload":     GetResPayload,
	"Namespaces":        NamespaceQuery,
	"Kinds":             KindQuery,
	"Queries":           QueryAvailableQueries,
	"GetResSummaryData": GetResSummaryData,
}

func Default() string {
	return "EventHeatMap"
}

func GetNamesOfQueries() []string {
	return []string{"EventHeatMap"}
}

func RunQuery(queryName string, params url.Values, tables typed.Tables, maxLookBack time.Duration, requestId string) ([]byte, error) {
	startTime, endTime, err := computeTimeRange(params, tables, maxLookBack)
	if err != nil {
		glog.Errorf("computeTimeRange failed with error: %v", err)
		return []byte{}, err
	}

	fn, ok := funcMap[queryName]
	if !ok {
		return []byte{}, fmt.Errorf("Query not found: " + queryName)
	}
	ret, err := fn(params, tables, startTime, endTime, requestId)
	if err != nil {
		glog.Errorf("Query %v failed with error: %v", queryName, err)
	}
	return ret, err
}
