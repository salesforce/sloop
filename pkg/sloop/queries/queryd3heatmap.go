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
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"net/url"
	"sort"
	"time"
)

const EmptyPartition = ""

type rawData struct {
	Events        map[typed.EventCountKey]*typed.ResourceEventCounts
	Resources     map[typed.ResourceSummaryKey]*typed.ResourceSummary
	WatchActivity map[typed.WatchActivityKey]*typed.WatchActivity
}

func EventHeatMap3Query(params url.Values, t typed.Tables, queryStartTime time.Time, queryEndTime time.Time, requestId string) ([]byte, error) {
	// Simple query of store for all rows in matching partitions (will include extra rows)
	rawRows, err := getRawDataFromStore(params, t, queryStartTime, queryEndTime, requestId)
	if err != nil {
		return nil, err
	}

	glog.Infof("reqId: %v EventHeatMap3Query read %v events, %v resources, and %v watch activity", requestId, len(rawRows.Events), len(rawRows.Resources), len(rawRows.WatchActivity))

	// Remove rows that don't fit in time range, and clip rows that go outside time range
	err = timeFilterResSumMap(rawRows.Resources, queryStartTime, queryEndTime)
	if err != nil {
		return []byte{}, err
	}
	err = timeFilterEventsMap(rawRows.Events, queryStartTime, queryEndTime)
	if err != nil {
		return []byte{}, err
	}
	timeFilterWatchActivityMap(rawRows.WatchActivity, queryStartTime, queryEndTime)

	glog.Infof("reqId: %v EventHeatMap3Query after filter %v events, %v resources, and %v watch activity", requestId, len(rawRows.Events), len(rawRows.Resources), len(rawRows.WatchActivity))

	// TODO: Get this 30 minutes from resync time from config
	err = adjustLastSeenTimeMap(rawRows.Resources, queryEndTime, 30*time.Minute)
	if err != nil {
		return []byte{}, err
	}

	// Simple one-to-one conversion of store resSum record to a d3 row
	mapResSumKeyToD3Gantt, err := resSumRowsToD3GanttMap(rawRows.Resources)
	if err != nil {
		return []byte{}, err
	}

	// add the event counts in as overlay
	mapResSumKeyToOverlay, err := eventCountsToOverlayMap(rawRows.Events)
	if err != nil {
		return []byte{}, err
	}
	err = mergeHeatmapWithResources(mapResSumKeyToD3Gantt, mapResSumKeyToOverlay)
	if err != nil {
		return []byte{}, err
	}

	// add the watch activity timestamps
	mapResSumKeyToWatchActivity, err := watchActivityToMap(rawRows.WatchActivity)
	if err != nil {
		return []byte{}, err
	}
	err = mergeHeatmapWithWatchActivity(mapResSumKeyToD3Gantt, mapResSumKeyToWatchActivity)
	if err != nil {
		return []byte{}, err
	}

	// Because overlays are grouped by minute, that minute might start before the resource was created or end after it finished
	// This moves the overlay start/end values so they are contained properly in the resource timeline
	outputRows := convertHeatmapToSlice(mapResSumKeyToD3Gantt)
	adjustOverlays(outputRows)

	outputRowValidation(outputRows, requestId)

	sortParam := params.Get(SortParam)
	outputRoot := TimelineRoot{
		Rows:    outputRows,
		ViewOpt: ViewOptions{Sort: sortParam},
	}

	bytes, err := json.MarshalIndent(outputRoot, "", " ")
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal json %v", err)
	}

	return bytes, nil
}

// Grab data from the store.  This will return rows from all partitions that intersect with startTime-endTime
// which will often include more rows that we need.
func getRawDataFromStore(params url.Values, t typed.Tables, startTime time.Time, endTime time.Time, requestId string) (rawData, error) {
	ret := rawData{}
	ret.Events = map[typed.EventCountKey]*typed.ResourceEventCounts{}
	ret.Resources = map[typed.ResourceSummaryKey]*typed.ResourceSummary{}
	ret.WatchActivity = map[typed.WatchActivityKey]*typed.WatchActivity{}

	err := t.Db().View(func(txn badgerwrap.Txn) error {
		var err2 error
		var stats typed.RangeReadStats
		ret.Events, stats, err2 = t.EventCountTable().RangeRead(txn, nil, paramEventCountSumFn(params), nil, startTime, endTime)
		if err2 != nil {
			return err2
		}
		stats.Log(requestId)

		ret.Resources, stats, err2 = t.ResourceSummaryTable().RangeRead(txn, nil, paramFilterResSumFn(params), nil, startTime, endTime)
		if err2 != nil {
			return err2
		}
		stats.Log(requestId)

		ret.WatchActivity, stats, err2 = t.WatchActivityTable().RangeRead(txn, nil, paramFilterWatchActivityFn(params), nil, startTime, endTime)
		if err2 != nil {
			return err2
		}
		stats.Log(requestId)

		return nil
	})
	if err != nil {
		return rawData{}, err
	}
	return ret, nil
}

// Last seen is either the last time a resource changed or when we got our last resync
// If the last seen time is close to the end of the query time we extend the lastSeenTime
// Otherwise it looks like resources stopped and events land outside the resource
func adjustLastSeenTime(resSum *typed.ResourceSummary, queryEndTime time.Time, resync time.Duration) error {
	// If the resource was deleted we should not change anything
	if resSum.DeletedAtEnd {
		return nil
	}
	lastTs, err := ptypes.Timestamp(resSum.LastSeen)
	if err != nil {
		return err
	}
	if lastTs.Add(resync).Equal(queryEndTime) || lastTs.Add(resync).After(queryEndTime) {
		resSum.LastSeen, err = ptypes.TimestampProto(queryEndTime)
		if err != nil {
			return err
		}
	}
	return nil
}

func adjustLastSeenTimeMap(resSumMap map[typed.ResourceSummaryKey]*typed.ResourceSummary, queryEndTime time.Time, resync time.Duration) error {
	for _, value := range resSumMap {
		err := adjustLastSeenTime(value, queryEndTime, resync)
		if err != nil {
			return err
		}
	}
	return nil
}

// Simple conversion of a resourceSummary row from the store to a d3Gantt row
func resSumRowToD3Gantt(key typed.ResourceSummaryKey, value *typed.ResourceSummary) (*TimelineRow, error) {
	startTs, err := ptypes.Timestamp(value.CreateTime)
	if err != nil {
		return nil, err
	}

	lastTs, err := ptypes.Timestamp(value.LastSeen)
	if err != nil {
		return nil, err
	}

	newRow := TimelineRow{
		Text:      key.Name,
		Kind:      key.Kind,
		StartDate: startTs.Unix(),
		EndDate:   lastTs.Unix(),
		Duration:  lastTs.Unix() - startTs.Unix(),
		Overlays:  []Overlay{},
		Namespace: key.Namespace,
	}
	return &newRow, nil
}

func takeNewest(left *TimelineRow, right *TimelineRow) *TimelineRow {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if left.EndDate > right.EndDate {
		return left
	} else {
		return right
	}
}

// Takes in all ResSum rows and returns a map of key to D3Gantt row
func resSumRowsToD3GanttMap(summaries map[typed.ResourceSummaryKey]*typed.ResourceSummary) (map[typed.ResourceSummaryKey]*TimelineRow, error) {
	result := map[typed.ResourceSummaryKey]*TimelineRow{}
	for key, value := range summaries {
		// This needs to be set to something constant so all resources come together
		key.PartitionId = EmptyPartition
		newRow, err := resSumRowToD3Gantt(key, value)
		if err != nil {
			return result, err
		}
		_, ok := result[key]
		if !ok {
			result[key] = newRow
		} else {
			// We have more than one entry for this resource summary row.  Take the newest
			result[key] = takeNewest(newRow, result[key])
		}
	}
	return result, nil
}

// Take a row from EventCountTable and extract the matching ResSum key and a slice of D3 Overlays
func eventCountRowToD3GanttOverlay(key typed.EventCountKey, value *typed.ResourceEventCounts) (typed.ResourceSummaryKey, []Overlay, error) {
	partitionStartTimestamp, _, err := untyped.GetTimeRangeForPartition(key.PartitionId)
	if err != nil {
		return typed.ResourceSummaryKey{}, []Overlay{}, err
	}

	refResSumKey := typed.NewResourceSummaryKey(partitionStartTimestamp, key.Kind, key.Namespace, key.Name, key.Uid)

	// This is a little ugly, but we want to group all the same reasons for the same bucket and sum them
	mapBucketMinToReasonToCount := map[int64]map[string]int32{}
	for unixMinute, eventCountMap := range value.MapMinToEvents {
		for reason, count := range eventCountMap.MapReasonToCount {
			_, ok := mapBucketMinToReasonToCount[unixMinute]
			if !ok {
				mapBucketMinToReasonToCount[unixMinute] = map[string]int32{}
			}
			mapBucketMinToReasonToCount[unixMinute][reason] += count
		}
	}

	mapBucketMinToText := map[int64]string{}
	for unixMinute, mapReasonToCount := range mapBucketMinToReasonToCount {
		// We need to get all reasons and sort them so we have deterministic output for easy unit tests
		sortedReasons := []string{}
		for reason, _ := range mapReasonToCount {
			sortedReasons = append(sortedReasons, reason)
		}
		sort.Strings(sortedReasons)

		text := ""
		for idx, reason := range sortedReasons {
			if idx != 0 {
				text += " "
			}
			text += fmt.Sprintf("%v:%v", reason, mapReasonToCount[reason])
			mapBucketMinToText[unixMinute] = text
		}
	}

	overlays := []Overlay{}

	for bucketMin, text := range mapBucketMinToText {
		newOverlay := Overlay{
			Text:      text,
			StartDate: bucketMin,
			Duration:  int64(60), // EventCounts are per minute
			EndDate:   time.Unix(bucketMin, 0).UTC().Add(time.Minute).Unix(),
		}
		overlays = append(overlays, newOverlay)
	}

	sort.Slice(overlays, func(i, j int) bool {
		return overlays[i].StartDate < overlays[j].StartDate
	})

	return *refResSumKey, overlays, nil
}

func eventCountsToOverlayMap(events map[typed.EventCountKey]*typed.ResourceEventCounts) (map[typed.ResourceSummaryKey][]Overlay, error) {
	retMap := map[typed.ResourceSummaryKey][]Overlay{}

	for key, value := range events {
		resSumRefKey, overlays, err := eventCountRowToD3GanttOverlay(key, value)
		if err != nil {
			return retMap, err
		}
		// In order for keys to join properly we need an empty partition ID
		resSumRefKey.PartitionId = EmptyPartition
		retMap[resSumRefKey] = append(retMap[resSumRefKey], overlays...)
	}
	return retMap, nil
}

func mergeHeatmapWithResources(resKeyToD3Map map[typed.ResourceSummaryKey]*TimelineRow, resKeyToOverlayMap map[typed.ResourceSummaryKey][]Overlay) error {
	// For some reason kubernetes Node objects have normal UUIDs, but events for a node have the node name filled in for involvedObject.UUID
	// This does not appear to be the case for other objects.  So we need a hack to make them match up properly

	for resKey, d3row := range resKeyToD3Map {
		if resKey.Kind == kubeextractor.NodeKind {
			resKey.Uid = resKey.Name
		}

		d3row.Overlays = resKeyToOverlayMap[resKey]
		if d3row.Overlays == nil {
			d3row.Overlays = []Overlay{}
		}
	}

	return nil
}

func watchActivityToMap(watchActivity map[typed.WatchActivityKey]*typed.WatchActivity) (map[typed.ResourceSummaryKey]typed.WatchActivity, error) {
	retMap := map[typed.ResourceSummaryKey]typed.WatchActivity{}

	for key, value := range watchActivity {
		partitionStartTimestamp, _, err := untyped.GetTimeRangeForPartition(key.PartitionId)
		if err != nil {
			return nil, err
		}

		resSumRefKey := *typed.NewResourceSummaryKey(partitionStartTimestamp, key.Kind, key.Namespace, key.Name, key.Uid)
		resSumRefKey.PartitionId = EmptyPartition // In order for keys to join properly we need an empty partition ID
		combined := typed.WatchActivity{}
		if existing, found := retMap[resSumRefKey]; found {
			combined = existing
		}
		combined.ChangedAt = append(combined.ChangedAt, value.ChangedAt...)
		combined.NoChangeAt = append(combined.NoChangeAt, value.NoChangeAt...)

		retMap[resSumRefKey] = combined
	}

	return retMap, nil
}

func mergeHeatmapWithWatchActivity(resKeyToD3Map map[typed.ResourceSummaryKey]*TimelineRow, watchActivity map[typed.ResourceSummaryKey]typed.WatchActivity) error {
	for resKey, d3row := range resKeyToD3Map {
		if activity, found := watchActivity[resKey]; found {
			d3row.ChangedAt = activity.ChangedAt
			d3row.NoChangeAt = activity.NoChangeAt
		} else {
			glog.Errorf("DEBUG: no activity - %v", resKey)
		}
	}

	return nil
}

func convertHeatmapToSlice(resKeyToD3Map map[typed.ResourceSummaryKey]*TimelineRow) []TimelineRow {
	var ret []TimelineRow

	for _, d3row := range resKeyToD3Map {
		ret = append(ret, *d3row)
	}

	return ret

}

// TODO: Add unit tests
// This adjusts overlays so they are always contained in the time range of the d3row.  This is needed because we bucket
// events by minute, but resources start and end somewhere inside the minute.
//
// Per-Minute Overlay:    |-- 1 --|-- 0 --|-- 4 --|...
// d3row:                    |-----------------|
// After adjustment:         |- 1 |-- 0 --|- 4 |
func adjustOverlays(rows []TimelineRow) {
	for _, d3row := range rows {
		// Fix all the start times for the overlay
		for idx, ol := range d3row.Overlays {
			tooEarlyMs := (d3row.StartDate - ol.StartDate)
			if tooEarlyMs > 0 && tooEarlyMs < 60*1000 {
				d3row.Overlays[idx].StartDate += tooEarlyMs
				d3row.Overlays[idx].Duration -= tooEarlyMs
				if d3row.Overlays[idx].Duration <= 0 {
					// We need to collapse this into a single time
					d3row.Overlays[idx].Duration = 0
					d3row.Overlays[idx].StartDate = d3row.Overlays[idx].EndDate
				}
			}
		}
	}
	for _, d3row := range rows {
		// Do a new loop for the end time
		for idx, ol := range d3row.Overlays {
			overMs := (ol.EndDate - d3row.EndDate)
			if overMs > 0 && overMs < 60*1000 {
				d3row.Overlays[idx].EndDate -= overMs
				d3row.Overlays[idx].Duration -= overMs
				if d3row.Overlays[idx].Duration <= 0 {
					d3row.Overlays[idx].Duration = 0
					d3row.Overlays[idx].EndDate = d3row.Overlays[idx].StartDate
				}
			}
		}
	}
}

// TODO: Add unit tests
func outputRowValidation(rows []TimelineRow, requestId string) {
	for _, d3row := range rows {

		if d3row.StartDate > d3row.EndDate {
			glog.Errorf("reqId: %v d3 row has start %v > end %v", requestId, d3row.StartDate, d3row.EndDate)
		}

		if d3row.StartDate+d3row.Duration != d3row.EndDate {
			glog.Errorf("reqId: %v d3 row times are inconsistent. start %v + duration %v != end %v.  Off by %v",
				requestId, d3row.StartDate, d3row.Duration, d3row.EndDate, d3row.StartDate+d3row.Duration-d3row.EndDate)
		}

		if d3row.Duration < 0 {
			glog.Errorf("reqId: %v d3row has negative duration %v", requestId, d3row.Duration)
		}

		for _, ol := range d3row.Overlays {
			if ol.StartDate > ol.EndDate {
				glog.Errorf("reqId: %v overlay has start %v > end %v", requestId, ol.StartDate, ol.EndDate)
			}
			if ol.StartDate+ol.Duration != ol.EndDate {
				glog.Errorf("reqId: %v overlay times are inconsistnet. start %v + duration %v != end %v.  Off by %v",
					requestId, ol.StartDate, ol.Duration, ol.EndDate, ol.StartDate+ol.Duration-ol.EndDate)
			}
			if ol.Duration < 0 {
				glog.Errorf("reqId: %v overlay has negative duration [%v] %v", requestId, ol.Text, ol.Duration)
			}

			if ol.StartDate < d3row.StartDate {
				tooEarlyMs := (d3row.StartDate - ol.StartDate)
				glog.Errorf("reqId: %v overlay is outside the bounds of d3 row.  OL Start %v < D3 Start %v.  Too early by %v ms\n",
					requestId, ol.StartDate, d3row.StartDate, tooEarlyMs)
			}
			if ol.EndDate > d3row.EndDate {
				tooLateMs := (ol.StartDate + ol.Duration) - (d3row.StartDate + d3row.Duration)
				glog.Errorf("reqId: %v overlay is outside the bounds of d3 row.  OL End %v > D3 End %v.  Runs over by %v ms\n",
					requestId, (ol.StartDate + ol.Duration), (d3row.StartDate + d3row.Duration),
					tooLateMs)
			}
		}
	}
}
