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
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"net/url"
	"strconv"
	"time"
)

const minLookback = 1 * time.Minute

// This computes a start and end time for a given query.  There are 2 ways this can be specified:
//
// Using "lookback":
//
//   We first find the endTime.  If we are looking at historic data, we use the end of the last partitions.  If
//   that is in the future, we use now().  We don't want to always use now() as that would prevent users from looking
//   at old data as it would get clipped by maxLookBack
//   StartTime is just endTime - lookback
//
// Using "start_time" and "end_time"
//
//   This is straight forward.  These are UTC Unix times
//
// TODO: If wall clock is in the middle of the newest partition min-max time we can use it
func computeTimeRange(params url.Values, tables typed.Tables, maxLookBack time.Duration) (time.Time, time.Time, error) {
	endOfTime := getEndOfTime(tables)
	return computeTimeRangeInternal(params, endOfTime, maxLookBack)
}

func computeTimeRangeInternal(params url.Values, endOfTime time.Time, maxLookBack time.Duration) (time.Time, time.Time, error) {
	lookBackVal := params.Get(LookbackParam)
	startTimeVal := params.Get(StartTimeParam)
	endTimeVal := params.Get(EndTimeParam)

	var computedStart time.Time
	var computedEnd time.Time
	var err error

	// Input validations
	if startTimeVal == "" && endTimeVal == "" && lookBackVal == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("Time range must be set with either [%v] or both of [%v,%v] but all 3 were empty", LookbackParam, StartTimeParam, EndTimeParam)
	}
	if lookBackVal != "" {
		if startTimeVal != "" || endTimeVal != "" {
			return time.Time{}, time.Time{}, fmt.Errorf("When [%v] is set, you can not set either of [%v,%v].  Got (%v,%v,%v) respectively", LookbackParam, StartTimeParam, EndTimeParam, lookBackVal, startTimeVal, endTimeVal)
		}
	} else {
		if (startTimeVal == "") || (endTimeVal == "") {
			return time.Time{}, time.Time{}, fmt.Errorf("Either %v and %v both need to be set or neither set.  Got (%v,%v) respectively", StartTimeParam, EndTimeParam, startTimeVal, endTimeVal)
		}
	}

	if lookBackVal != "" {
		computedEnd = endOfTime
		lookbackRange, err := getDurationFromLookback(lookBackVal)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		computedStart = computedEnd.Add(-1 * lookbackRange)
	} else {
		computedStart, computedEnd, err = getTimeRangeFromStartEnd(startTimeVal, endTimeVal)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	// If the time range ends beyond endOfTime shift it back
	if computedEnd.After(endOfTime) {
		shiftBy := computedEnd.Sub(endOfTime)
		computedStart = computedStart.Add(-1 * shiftBy)
		computedEnd = computedEnd.Add(-1 * shiftBy)
	}

	// If the time range is too small, increase it
	if computedEnd.Sub(computedStart) < minLookback {
		computedStart = computedEnd.Add(-1 * minLookback)
	}

	if computedEnd.Sub(computedStart) > maxLookBack {
		computedStart = computedEnd.Add(-1 * maxLookBack)
	}

	return computedStart, computedEnd, nil

}

// This looks at our store, and if it has data finds the newest partition, then finds the end time of that
// But if that is in the future we return now
// This bit of logic is needed for queries with a lookback to determine a good end time
func getEndOfTime(tables typed.Tables) time.Time {
	now := time.Now()

	ok, _, maxPartition, err := tables.GetMinAndMaxPartition()
	if err != nil || !ok {
		if err != nil {
			glog.Errorf("Error getting MinAndMaxPartition: %v", err)
		}
		return now
	}

	_, endTimeOfNewestPartition, err := untyped.GetTimeRangeForPartition(maxPartition)
	if err != nil {
		glog.Errorf("Error getting MinAndMaxPartition: %v", err)
		return now
	}

	// The newest partition ends in the future, so use now instead
	if endTimeOfNewestPartition.After(now) {
		return now
	} else {
		return endTimeOfNewestPartition
	}
}

func getDurationFromLookback(lookbackVal string) (time.Duration, error) {
	queryDuration, err := time.ParseDuration(lookbackVal)
	if err != nil {
		glog.Errorf("Invalid lookback param: %v.  err: %v", lookbackVal, err)
		return 0, err
	}

	return queryDuration, nil
}

func getTimeRangeFromStartEnd(startTimeStr string, endTimeStr string) (time.Time, time.Time, error) {
	var start, end time.Time
	var err error
	start, err = parseUnixTimeString(startTimeStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err = parseUnixTimeString(endTimeStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, end, nil
}

func parseUnixTimeString(unixStr string) (time.Time, error) {
	unixNum, err := strconv.ParseInt(unixStr, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(unixNum, 0).UTC(), nil
}

// This extracts time info from the ResourceSummary value and checks if it overlaps with the query time range.
// If outside the range it returns false
// If fully inside the range it returns true and nothing is modified
// If partially in the range it clips off the parts that are outside the range and returns true
func timeFilterResSumValue(value *typed.ResourceSummary, queryStartTime time.Time, queryEndTime time.Time) (bool, error) {
	startTs, err := ptypes.Timestamp(value.CreateTime)
	if err != nil {
		return false, err
	}
	lastTs, err := ptypes.Timestamp(value.LastSeen)
	if err != nil {
		return false, err
	}
	if startTs.After(queryEndTime) || lastTs.Before(queryStartTime) {
		// This will not show up anyways, lets filter it
		return false, nil
	}
	if startTs.Before(queryStartTime) {
		value.CreateTime, err = ptypes.TimestampProto(queryStartTime)
		if err != nil {
			return false, err
		}
	}
	if lastTs.After(queryEndTime) {
		value.LastSeen, err = ptypes.TimestampProto(queryEndTime)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func timeFilterResSumMap(resSumMap map[typed.ResourceSummaryKey]*typed.ResourceSummary, queryStartTime time.Time, queryEndTime time.Time) error {
	for key, value := range resSumMap {
		keep, err := timeFilterResSumValue(value, queryStartTime, queryEndTime)
		if err != nil {
			return err
		}
		if !keep {
			delete(resSumMap, key)
		}
	}
	return nil
}

func timeFilterEventValue(value *typed.ResourceEventCounts, queryStartTime time.Time, queryEndTime time.Time) (bool, error) {
	// TODO: Event values have a map of minute within a partition to a count of events
	// We need to compute the time for each minute and rewrite the value accordingly
	return true, nil
}

func timeFilterEventsMap(events map[typed.EventCountKey]*typed.ResourceEventCounts, queryStartTime time.Time, queryEndTime time.Time) error {
	for key, value := range events {
		keep, err := timeFilterEventValue(value, queryStartTime, queryEndTime)
		if err != nil {
			return err
		}
		if !keep {
			delete(events, key)
		}
	}
	return nil
}

func timeFilterWatchActivityOccurrences(occurrences []int64, queryStartTime time.Time, queryEndTime time.Time) []int64 {
	start := queryStartTime.Unix()
	end := queryEndTime.Unix()

	filtered := make([]int64, 0, len(occurrences))
	for _, when := range occurrences {
		if when >= start && when <= end {
			filtered = append(filtered, when)
		}
	}

	return filtered
}

func timeFilterWatchActivity(activity *typed.WatchActivity, queryStartTime time.Time, queryEndTime time.Time) *typed.WatchActivity {
	activity.ChangedAt = timeFilterWatchActivityOccurrences(activity.ChangedAt, queryStartTime, queryEndTime)
	activity.NoChangeAt = timeFilterWatchActivityOccurrences(activity.NoChangeAt, queryStartTime, queryEndTime)
	return activity
}

func timeFilterWatchActivityMap(activityMap map[typed.WatchActivityKey]*typed.WatchActivity, queryStartTime time.Time, queryEndTime time.Time) {
	for key, value := range activityMap {
		filtered := timeFilterWatchActivity(value, queryStartTime, queryEndTime)
		if len(filtered.NoChangeAt) == 0 && len(filtered.ChangedAt) == 0 {
			delete(activityMap, key)
		} else {
			activityMap[key] = filtered
		}
	}
}
