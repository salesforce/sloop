/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package storemanager

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/salesforce/sloop/pkg/sloop/common"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/spf13/afero"
	"math"
	"sync"
	"time"
)

var (
	metricGcRunCount                   = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_gc_run_count"})
	metricGcSuccessCount               = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_gc_success_count"})
	metricGcFailedCount                = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_gc_failed_count"})
	metricGcLatency                    = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_latency_sec"})
	metricGcRunning                    = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_running"})
	metricGcCleanUpPerformed           = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_cleanup_performed"})
	metricGcDeletedNumberOfKeys        = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_deleted_num_of_keys"})
	metricGcNumberOfKeysToDelete       = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_num_of_keys_to_delete"})
	metricGcDeletedNumberOfKeysByTable = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "sloop_deleted_keys_by_table"}, []string{"table"})
	metricAgeOfMinimumPartition        = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_age_of_minimum_partition_hr"})
	metricAgeOfMaximumPartition        = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_age_of_maximum_partition_hr"})
	metricValueLogGcRunCount           = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_valueLoggc_run_count"})
	metricValueLogGcLatency            = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_valueLoggc_latency_sec"})
	metricValueLogGcRunning            = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_valueLoggc_running"})
	metricTotalNumberOfKeys            = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_total_number_of_keys"})
)

type Config struct {
	StoreRoot          string
	Freq               time.Duration
	TimeLimit          time.Duration
	SizeLimitBytes     int
	BadgerDiscardRatio float64
	BadgerVLogGCFreq   time.Duration
	DeletionBatchSize  int
	GCThreshold        float64
}

type StoreManager struct {
	tables   typed.Tables
	fs       *afero.Afero
	testMode bool
	sleeper  *SleepWithCancel
	wg       *sync.WaitGroup
	done     bool
	donelock *sync.Mutex
	config   *Config
	stats    *storeStats
}

func NewStoreManager(tables typed.Tables, config *Config, fs *afero.Afero) *StoreManager {
	return &StoreManager{
		tables:   tables,
		fs:       fs,
		sleeper:  NewSleepWithCancel(),
		wg:       &sync.WaitGroup{},
		done:     false,
		donelock: &sync.Mutex{},
		config:   config,
	}
}

func (sm *StoreManager) isDone() bool {
	sm.donelock.Lock()
	defer sm.donelock.Unlock()
	return sm.done
}

func (sm *StoreManager) Start() {
	go sm.gcLoop()
	go sm.vlogGcLoop()
}

func (sm *StoreManager) gcLoop() {
	sm.wg.Add(1)
	defer sm.wg.Done()
	defer metricGcRunning.Set(0)
	for {
		if sm.isDone() {
			glog.Infof("Store manager main loop exiting")
			return
		}

		var beforeGCStats = sm.refreshStats()

		metricGcRunCount.Inc()
		before := time.Now()
		metricGcRunning.Set(1)
		cleanUpPerformed, numOfDeletedKeys, numOfKeysToDelete, err := doCleanup(sm.tables, sm.config.TimeLimit, sm.config.SizeLimitBytes, sm.stats, sm.config.DeletionBatchSize, sm.config.GCThreshold)
		metricGcCleanUpPerformed.Set(common.BoolToFloat(cleanUpPerformed))
		metricGcDeletedNumberOfKeys.Set(float64(numOfDeletedKeys))
		metricGcNumberOfKeysToDelete.Set(float64(numOfKeysToDelete))
		metricGcRunning.Set(0)
		if err == nil {
			metricGcSuccessCount.Inc()
		} else {
			metricGcFailedCount.Inc()
		}
		metricGcLatency.Set(time.Since(before).Seconds())
		glog.Infof("GC finished in %v with error '%v'.  Next run in %v", time.Since(before), err, sm.config.Freq)

		var afterGCEnds = sm.refreshStats()
		var deltaStats = getDeltaStats(beforeGCStats, afterGCEnds)
		emitGCMetrics(deltaStats)
		sm.sleeper.Sleep(sm.config.Freq)
	}
}

func (sm *StoreManager) vlogGcLoop() {
	// Its up to us to trigger the Badger value log GC.
	// See https://github.com/dgraph-io/badger#garbage-collection
	sm.wg.Add(1)
	defer sm.wg.Done()
	defer metricValueLogGcRunning.Set(0)
	for {
		if sm.isDone() {
			glog.Infof("ValueLogGC loop exiting")
			return
		}

		var beforeGCStats = sm.refreshStats()
		for {
			before := time.Now()
			metricValueLogGcRunning.Set(1)
			err := sm.tables.Db().RunValueLogGC(sm.config.BadgerDiscardRatio)
			metricValueLogGcRunning.Set(0)
			metricValueLogGcRunCount.Add(1)
			metricValueLogGcLatency.Set(time.Since(before).Seconds())
			glog.Infof("RunValueLogGC(%v) run took %v and returned %q", sm.config.BadgerDiscardRatio, time.Since(before), err)
			if err != nil {
				break
			}
			var afterGCEnds = sm.refreshStats()
			var deltaStats = getDeltaStats(beforeGCStats, afterGCEnds)
			emitGCMetrics(deltaStats)
		}
		sm.sleeper.Sleep(sm.config.BadgerVLogGCFreq)
	}
}

func (sm *StoreManager) Shutdown() {
	glog.Infof("Starting store manager shutdown")
	sm.donelock.Lock()
	sm.done = true
	sm.donelock.Unlock()
	sm.sleeper.Cancel()
	sm.wg.Wait()
}

func (sm *StoreManager) refreshStats() *storeStats {
	// On startup we have 2 routines trying to do this at the same time
	// If we have fresh results its good enough
	if sm.stats != nil && time.Since(sm.stats.timestamp) < time.Second {
		return sm.stats
	}
	sm.stats = generateStats(sm.config.StoreRoot, sm.tables.Db(), sm.fs)
	emitMetrics(sm.stats)
	return sm.stats
}

func doCleanup(tables typed.Tables, timeLimit time.Duration, sizeLimitBytes int, stats *storeStats, deletionBatchSize int, gcThreshold float64) (bool, int64, int64, error) {

	ok, minPartition, maxPartition, err := tables.GetMinAndMaxPartition()
	if err != nil {
		return false, 0, 0, fmt.Errorf("failed to get min partition : %s, max partition: %s, err:%v", minPartition, maxPartition, err)
	}
	if !ok {
		return false, 0, 0, nil
	}

	minPartitionAge, err := untyped.GetAgeOfPartitionInHours(minPartition)
	if err == nil {
		metricAgeOfMinimumPartition.Set(minPartitionAge)
	}

	maxPartitionAge, err := untyped.GetAgeOfPartitionInHours(maxPartition)
	if err == nil {
		metricAgeOfMaximumPartition.Set(maxPartitionAge)
	}

	var totalNumOfDeletedKeys int64 = 0
	var totalNumOfKeysToDelete int64 = 0
	anyCleanupPerformed := false
	minPartitionStartTime, err := untyped.GetTimeForPartition(minPartition)
	if err != nil {
		return false, 0, 0, err
	}

	var numOfKeysToDeleteForFileSizeCondition int64 = 0
	isFileSizeConditionMet, garbageCollectionRatio := cleanUpFileSizeCondition(stats, sizeLimitBytes, gcThreshold)

	if isFileSizeConditionMet {
		numOfKeysToDeleteForFileSizeCondition = getNumberOfKeysToDelete(tables.Db(), garbageCollectionRatio)
	}

	beforeGCTime := time.Now()
	for cleanUpTimeCondition(minPartition, maxPartition, timeLimit) || numOfKeysToDeleteForFileSizeCondition > 0 {

		numOfDeletedKeysforPrefix, numOfKeysToDeleteForPrefix, errMessages := deletePartition(minPartition, tables, deletionBatchSize)
		totalNumOfDeletedKeys += int64(numOfDeletedKeysforPrefix)
		totalNumOfKeysToDelete += int64(numOfKeysToDeleteForPrefix)
		anyCleanupPerformed = true
		minPartitionStartTime = minPartitionStartTime.Add(untyped.GetPartitionDuration())
		minPartition = untyped.GetPartitionId(minPartitionStartTime)

		minPartitionAge, err := untyped.GetAgeOfPartitionInHours(minPartition)
		if err != nil || minPartitionAge < 0 {
			return false, totalNumOfDeletedKeys, totalNumOfKeysToDelete, fmt.Errorf("minimun partition age: %f cannot be less than zero", minPartitionAge)
		}

		metricAgeOfMinimumPartition.Set(minPartitionAge)
		if len(errMessages) != 0 {
			var errMsg string
			for _, er := range errMessages {
				errMsg += er + ","
			}
			return false, totalNumOfDeletedKeys, totalNumOfKeysToDelete, fmt.Errorf(errMsg)
		}

		glog.Infof("Deleted Number of keys so far: %v ", totalNumOfDeletedKeys)
		if numOfKeysToDeleteForFileSizeCondition > totalNumOfDeletedKeys {
			numOfKeysToDeleteForFileSizeCondition -= totalNumOfDeletedKeys
		} else {
			// Deleted number of keys is greater or equal. We have reached the required deletion.
			numOfKeysToDeleteForFileSizeCondition = 0
		}

		glog.Infof("Remaining number of keys to delete: %v  ", numOfKeysToDeleteForFileSizeCondition)
	}

	elapsed := time.Since(beforeGCTime)
	glog.Infof("Deletion of prefixes took %v and removed %d keys with error: %v", elapsed, totalNumOfDeletedKeys, err)

	beforeDropPrefix := time.Now()

	// dropping prefix to force compression
	err = tables.Db().DropPrefix([]byte{})
	glog.Infof("Drop prefix took %v with error: %v", time.Since(beforeDropPrefix), err)
	return anyCleanupPerformed, totalNumOfDeletedKeys, totalNumOfKeysToDelete, nil
}

func deletePartition(minPartition string, tables typed.Tables, deletionBatchSize int) (int, int, []string) {
	totalNumOfDeletedKeysforPrefix := 0
	totalNumOfKeysToDeleteForPrefix := 0
	numOfDeletedKeysforPrefix := 0
	numOfKeysToDeleteForPrefix := 0

	partStart, partEnd, err := untyped.GetTimeRangeForPartition(minPartition)
	glog.Infof("GC removing partition %q with data from %v to %v (err %v)", minPartition, partStart, partEnd, err)
	var errMessages []string
	for _, tableName := range tables.GetTableNames() {
		prefix := fmt.Sprintf("/%s/%s", tableName, minPartition)
		start := time.Now()
		err, numOfDeletedKeysforPrefix, numOfKeysToDeleteForPrefix = common.DeleteKeysWithPrefix([]byte(prefix), tables.Db(), deletionBatchSize)
		metricGcDeletedNumberOfKeysByTable.WithLabelValues(fmt.Sprintf("%v", tableName)).Set(float64(numOfDeletedKeysforPrefix))
		elapsed := time.Since(start)
		glog.Infof("Call to DeleteKeysWithPrefix(%v) took %v and removed %d keys with error: %v", prefix, elapsed, numOfDeletedKeysforPrefix, err)
		if err != nil {
			errMessages = append(errMessages, fmt.Sprintf("failed to cleanup with min key: %s, elapsed: %v,err: %v,", prefix, elapsed, err))
		}

		totalNumOfDeletedKeysforPrefix += numOfDeletedKeysforPrefix
		totalNumOfKeysToDeleteForPrefix += numOfKeysToDeleteForPrefix
	}

	return totalNumOfDeletedKeysforPrefix, totalNumOfKeysToDeleteForPrefix, errMessages
}

func cleanUpTimeCondition(minPartition string, maxPartition string, timeLimit time.Duration) bool {
	oldestTime, _, err := untyped.GetTimeRangeForPartition(minPartition)
	if err != nil {
		glog.Error(err)
		return false
	}
	_, latestTime, err := untyped.GetTimeRangeForPartition(maxPartition)
	if err != nil {
		glog.Error(err)
		return false
	}

	timeDiff := latestTime.Sub(oldestTime)
	if timeDiff > timeLimit {
		glog.Infof("Start cleaning up because current time diff: %v exceeds time limit: %v", timeDiff, timeLimit)
		return true
	}

	glog.V(2).Infof("Can not clean up, wait until clean up time gap: %v exceeds time limit: %v yet", timeDiff, timeLimit)
	return false
}

func cleanUpFileSizeCondition(stats *storeStats, sizeLimitBytes int, gcThreshold float64) (bool, float64) {

	// gcthreshold is the threshold when reached would trigger the garbage collection. Its because we want to proactively start GC when the size limit is about to hit.
	sizeThreshold := gcThreshold * float64(sizeLimitBytes)
	currentDiskSize := float64(stats.DiskSizeBytes)
	if currentDiskSize > sizeThreshold {
		glog.Infof("Start cleaning up because current file size: %v exceeds file size threshold: %v", stats.DiskSizeBytes, sizeThreshold)

		garbageCollectionRatio := (currentDiskSize - sizeThreshold) / currentDiskSize
		return true, garbageCollectionRatio
	}

	glog.V(2).Infof("Can not clean up, disk size: %v is not exceeding size limit: %v yet", stats.DiskSizeBytes, uint64(sizeLimitBytes))
	return false, 0.0
}

func getNumberOfKeysToDelete(db badgerwrap.DB, garbageCollectionRatio float64) int64 {
	totalKeyCount := float64(common.GetTotalKeyCount(db))
	metricTotalNumberOfKeys.Set(totalKeyCount)

	if garbageCollectionRatio <= 0 || garbageCollectionRatio > 1 {
		// print float here and below
		glog.V(2).Infof("Garbage collection ratio out of bounds. Unexpected ratio: %f", garbageCollectionRatio)
		return 0
	}

	keysToDelete := garbageCollectionRatio * totalKeyCount
	return int64(math.Ceil(keysToDelete))
}
