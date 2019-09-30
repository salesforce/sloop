/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package queries

import (
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"net/url"
	"time"
)

// This computes a start and end time for a given query.  If user specifies a time duration we use it, otherwise
// we use maxLookBack from config.  For the end time if not specified we want to use newest data in the store.
// That way we can look at old data sets without clipping.  We know the min and max time of the newest partition
// but we dont really know the newest record within that range.  For now just use end of newest partiton, but
// we will improve that later.
// TODO: Add unit tests
// TODO: If wall clock is in the middle of the newest partition min-max time we can use it
func computeTimeRange(params url.Values, tables typed.Tables, maxLookBack time.Duration) (time.Time, time.Time) {
	now := time.Now()

	// If web request specifies a valid lookback use that, else use the config for the store
	queryDuration := maxLookBack
	queryLookBack := params.Get(LookbackParam)
	if queryLookBack != "" {
		var err error
		queryDuration, err = time.ParseDuration(queryLookBack)
		if err != nil {
			glog.Errorf("Invalid lookback param: %v.  err: %v", queryLookBack, err)
		}
	}
	if queryDuration < 10*time.Minute || queryDuration > maxLookBack {
		queryDuration = maxLookBack
	}

	// Find the end of the newest store partition and use that as endTime
	ok, _, maxPartition, err := tables.GetMinAndMaxPartition()
	if err != nil || !ok {
		if err != nil {
			glog.Errorf("Error getting MinAndMaxPartition: %v", err)
		}
		// Store is broken or has no data.  Best we can do is now - queryDuration
		return now.Add(-1 * queryDuration), now
	}

	_, endTimeOfNewestPartition, err := untyped.GetTimeRangeForPartition(maxPartition)
	if err != nil {
		glog.Errorf("Error getting MinAndMaxPartition: %v", err)
		return now.Add(-1 * queryDuration), now
	}

	// The newest partition ends in the future, so use now instead
	if endTimeOfNewestPartition.After(now) {
		return now.Add(-1 * queryDuration), now
	}

	return endTimeOfNewestPartition.Add(-1 * queryDuration), endTimeOfNewestPartition
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
