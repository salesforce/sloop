/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package processing

import (
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"math"
	"time"
)

// TODO: We are only looking for the previous event in the current partiton, but we need to look back in cases where we cross the boundary

func updateEventCountTable(
	tables typed.Tables,
	txn badgerwrap.Txn,
	watchRec *typed.KubeWatchResult,
	metadata *kubeextractor.KubeMetadata,
	involvedObject *kubeextractor.KubeInvolvedObject,
	maxLookback time.Duration) error {
	if watchRec.Kind != kubeextractor.EventKind {
		glog.V(7).Infof("Skipping event processing for %v", watchRec.Kind)
		return nil
	}

	prevEventInfo, err := getPreviousEventInfo(tables, txn, watchRec.Timestamp, watchRec.Kind, metadata.Namespace, metadata.Name)
	if err != nil {
		return errors.Wrap(err, "Could not get event info for previous event instance")
	}

	newEventInfo, err := kubeextractor.ExtractEventInfo(watchRec.Payload)
	if err != nil {
		return errors.Wrap(err, "Could not extract reason")
	}

	computedFirstTs, computedLastTs, computedCount := computeEventsDiff(prevEventInfo, newEventInfo)
	if computedCount == 0 {
		return nil
	}

	// Truncate long-lived events to available partitions
	// This avoids filling in data that will go beyond the current time range
	ok, minPartition, maxPartition := tables.GetMinAndMaxPartitionWithTxn(txn)
	if err != nil || !ok {
		return err
	}

	_, minPartitionEndTime, err := untyped.GetTimeRangeForPartition(minPartition)
	if err != nil && minPartition != "" {
		return err
	}

	maxPartitionStartTime, maxPartitionEndTime, err := untyped.GetTimeRangeForPartition(maxPartition)
	if err != nil && maxPartition != "" {
		return err
	}

	computedFirstTs, computedLastTs, computedCount = adjustForAvailablePartitions(computedFirstTs, computedLastTs, computedCount, minPartitionEndTime, maxPartitionStartTime, maxPartitionEndTime)

	eventCountByMinute := spreadOutEvents(computedFirstTs, computedLastTs, computedCount)

	err = storeMinutes(tables, txn, eventCountByMinute, involvedObject.Kind, involvedObject.Namespace, involvedObject.Name, involvedObject.Uid, newEventInfo.Reason, newEventInfo.Type)
	if err != nil {
		return err
	}

	return nil
}

func storeMinutes(tables typed.Tables, txn badgerwrap.Txn, minToCount map[int64]int, kind string, namespace string, name string, uid string, reason string, severity string) error {
	// We have event counts over different timestamps, which can be in different partitions.  But we want to do all
	// the work for the same partition in one round trip.

	mapPartToTimeToCount := map[string]map[int64]int{}
	for unixTime, count := range minToCount {
		thisTs := time.Unix(unixTime, 0)
		partitionId := untyped.GetPartitionId(thisTs)
		_, ok := mapPartToTimeToCount[partitionId]
		if !ok {
			mapPartToTimeToCount[partitionId] = map[int64]int{}
		}

		mapPartToTimeToCount[partitionId][unixTime] = count
	}

	for _, thisPartMap := range mapPartToTimeToCount {
		for unixTime, count := range thisPartMap {

			key := typed.NewEventCountKey(time.Unix(unixTime, 0).UTC(), kind, namespace, name, uid)

			eventRecord, err := tables.EventCountTable().GetOrDefault(txn, key.String())
			if err != nil {
				return errors.Wrap(err, "Could not get event record")
			}

			// some event records were being returned with nil MapMinToEvents, this was causing runtime exception. Adding a TODO to investigate why these kind of records exist.
			if eventRecord == nil || eventRecord.MapMinToEvents == nil {
				return errors.Wrap(err, "Either retrieved event record  is nil or its  MapMinToEvents is nil")
			}

			if _, ok := eventRecord.MapMinToEvents[unixTime]; !ok {
				eventRecord.MapMinToEvents[unixTime] = &typed.EventCounts{MapReasonToCount: make(map[string]int32)}
			}

			eventRecord.MapMinToEvents[unixTime].MapReasonToCount[reason+":"+severity] += int32(count)

			err = tables.EventCountTable().Set(txn, key.String(), eventRecord)
			if err != nil {
				return errors.Wrap(err, "Failed to put")
			}
		}
	}
	return nil
}

func distributeValue(value int, buckets int) []int {
	if buckets == 0 {
		return []int{}
	}
	ret := []int{}
	for pos := 0; pos < buckets; pos += 1 {
		thisVal := value / buckets
		if value%buckets > pos {
			thisVal += 1
		}
		ret = append(ret, thisVal)
	}
	return ret
}

// TODO: Do this the right way so the totals always match.  This is a placeholder solution
// TODO: Figure out proper way to round this
func spreadOutEvents(firstTs time.Time, lastTs time.Time, count int) map[int64]int {
	ret := map[int64]int{}
	firstRound := firstTs.Round(time.Minute)
	lastRound := lastTs.Round(time.Minute)
	// It all happened in the same minute
	if firstRound == lastRound {
		ret[firstRound.Unix()] = count
		return ret
	}
	numMinutes := int(math.Ceil(lastRound.Sub(firstRound).Minutes()))
	if numMinutes < 1 {
		numMinutes = 1
	}
	counts := distributeValue(count, numMinutes)
	thisMinute := firstRound
	for idx := 0; idx < numMinutes; idx += 1 {
		if counts[idx] > 0 {
			ret[thisMinute.Unix()] = counts[idx]
		}
		thisMinute = thisMinute.Add(time.Minute)
	}
	return ret
}

func getPreviousEventInfo(tables typed.Tables, txn badgerwrap.Txn, ts *timestamp.Timestamp, kind string, namespace string, name string) (*kubeextractor.EventInfo, error) {
	// Find the most recent copy of this event in the store so we can figure out what is new
	prevWatch, err := getLastKubeWatchResult(tables, txn, ts, kind, namespace, name)
	if err != nil {
		return nil, err
	}
	if prevWatch == nil {
		return nil, nil
	}

	return kubeextractor.ExtractEventInfo(prevWatch.Payload)
}

// Subtract old events from new events
func computeEventsDiff(prevEventInfo *kubeextractor.EventInfo, newEventInfo *kubeextractor.EventInfo) (time.Time, time.Time, int) {
	// First time we are seeing this event, so just return it:
	//
	// Old: nil
	// New: |----- Count: 50 ---|
	if prevEventInfo == nil {
		return newEventInfo.FirstTimestamp, newEventInfo.LastTimestamp, newEventInfo.Count
	}

	// Old event does not overlap, so return the new event:
	//
	// Old: |--- Count: 2 --|
	// New:                    |-- Count: 1 --|
	if prevEventInfo.LastTimestamp.Before(newEventInfo.FirstTimestamp) {
		return newEventInfo.FirstTimestamp, newEventInfo.LastTimestamp, newEventInfo.Count
	}

	// This is a duplicate or old event, so just return count=0 (no new events)
	//
	// Old: |----- Count: 50 ---|
	// New: |----- Count: 50 ---|
	//
	// or possibly this strange one:
	//
	// Old: |----- Count: 55 --------|
	// New: |----- Count: 50 ---|
	if prevEventInfo.LastTimestamp.Equal(newEventInfo.LastTimestamp) || prevEventInfo.LastTimestamp.After(newEventInfo.LastTimestamp) {
		return time.Time{}, time.Time{}, 0
	}

	// New and old events start at the same time, but new ends later.  This will happen all the time, and we subtract the old from new
	//
	// Old: |----- Count: 50 ---|
	// New: |------------- Count: 62 -----|
	// So we return:
	//                          |-- 12 ---|
	if prevEventInfo.FirstTimestamp == newEventInfo.FirstTimestamp {
		if newEventInfo.Count < prevEventInfo.Count {
			// This should not happen!
			glog.Errorf("New event has a lower count than previous event wth same start time! Old %v New %v", prevEventInfo, newEventInfo)
			return time.Time{}, time.Time{}, 0
		}
		return prevEventInfo.LastTimestamp, newEventInfo.LastTimestamp, newEventInfo.Count - prevEventInfo.Count
	}

	// If we reach here, we have partially overlapping event ranges like this which should NOT happen.
	// Figure out the percent overlap, and reduce the old count by that amount.  This is the best approximation we can do.
	// Old:   |---- count: 123 -----|
	// New:              |----- count: 4235 ----|
	glog.Errorf("Encountered partially overlapping events.  Attempting to guess new count")
	oldSeconds := prevEventInfo.LastTimestamp.Sub(prevEventInfo.FirstTimestamp).Seconds()
	overlapSeconds := prevEventInfo.LastTimestamp.Sub(newEventInfo.FirstTimestamp).Seconds()
	if oldSeconds <= 0 {
		// Should not happen, but dont want a divide by zero
		return time.Time{}, time.Time{}, 0
	}
	pctOverlap := float64(overlapSeconds) / float64(oldSeconds)
	newCount := newEventInfo.Count - int(float64(prevEventInfo.Count)*pctOverlap)
	if newCount < 0 {
		newCount = 0
	}
	return prevEventInfo.LastTimestamp, newEventInfo.LastTimestamp, newCount
}

// Returns a new firstTs, lastTs and Count
func adjustForAvailablePartitions(firstTs time.Time, lastTs time.Time, count int, minPartitionEndTime time.Time, maxPartitionStartTime time.Time, maxPartitionEndTime time.Time) (time.Time, time.Time, int) {
	beginTruncateTs := minPartitionEndTime
	endTruncateTs := maxPartitionEndTime

	// If begin and end are the same, there is only one partition.
	// Allow event count to add to only this partition.
	if beginTruncateTs == endTruncateTs {
		beginTruncateTs = maxPartitionStartTime
	}

	// There is no overlap with begin and end, there is no event count to return
	if lastTs.Before(beginTruncateTs) || firstTs.After(endTruncateTs) {
		return beginTruncateTs, endTruncateTs, 0
	}

	totalSeconds := lastTs.Sub(firstTs).Seconds()
	secondsToKeep := totalSeconds

	// if firstTs is before beginTruncateTs. Truncate to beginTruncateTs.
	// else set beginTruncateTs to firstTs
	if firstTs.Before(beginTruncateTs) {
		secondsToKeep = secondsToKeep - beginTruncateTs.Sub(firstTs).Seconds()
	} else {
		beginTruncateTs = firstTs
	}

	// if lastTs is after endTruncateTs. Truncate to endTruncateTs
	// else set endTruncateTs to lastTs
	if lastTs.After(endTruncateTs) {
		secondsToKeep = secondsToKeep - lastTs.Sub(endTruncateTs).Seconds()
	} else {
		endTruncateTs = lastTs
	}

	pctEventsToKeep := secondsToKeep / totalSeconds
	return beginTruncateTs, endTruncateTs, int(float64(count) * pctEventsToKeep)
}
