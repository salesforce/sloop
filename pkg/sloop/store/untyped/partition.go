/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package untyped

import (
	"fmt"
	"strconv"
	"time"
)

// For now we want the ability to try different durations, and this can not change during runtime
// Keys need access to GetPartitionId() which needs this value, and we dont want to pass around config everywhere
// that deals with keys.
// TODO: Later when we figure out an ideal partition duration lets remove it from config so users dont change it
// and end up with data that does not match the business logic
var partitionDuration time.Duration

// Partitions need to be in lexicographical sorted order, so zero pad to 12 digits
func GetPartitionId(timestamp time.Time) string {
	if partitionDuration == time.Hour {
		rounded := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), timestamp.Hour(), 0, 0, 0, timestamp.Location())
		return fmt.Sprintf("%012d", uint64(rounded.Unix()))
	} else if partitionDuration == 24*time.Hour {
		rounded := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location())
		return fmt.Sprintf("%012d", uint64(rounded.Unix()))
	} else {
		panic("Invalid partition duration")
	}
}

func GetTimeForPartition(partitionId string) (time.Time, error) {
	partInt, err := strconv.ParseInt(partitionId, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	partitionTime := time.Unix(partInt, 0).UTC()
	return partitionTime, nil
}

func GetTimeRangeForPartition(partitionId string) (time.Time, time.Time, error) {
	oldestTime, err := GetTimeForPartition(partitionId)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	var newestTime time.Time
	if partitionDuration == time.Hour {
		newestTime = oldestTime.Add(time.Hour)
	} else if partitionDuration == 24*time.Hour {
		newestTime = oldestTime.Add(24 * time.Hour)
	} else {
		panic("Invalid partition duration")
	}
	return oldestTime, newestTime, nil
}

func GetAgeOfPartitionInHours(partitionId string) (float64, error) {
	timeForPartition, err := GetTimeForPartition(partitionId)
	if err != nil {
		return -1, err
	}

	nanosecondsInAnHour := time.Duration(60 * 60 * 1000000000)
	return float64(time.Now().Sub(timeForPartition) / nanosecondsInAnHour), nil
}

func TestHookSetPartitionDuration(partDuration time.Duration) {
	partitionDuration = partDuration
}

func GetPartitionDuration() time.Duration {
	return partitionDuration
}
