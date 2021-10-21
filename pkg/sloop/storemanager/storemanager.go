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
	metricDropPrefixLatency            = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_drop_prefix_sec"})
	metricGcRunning                    = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_running"})
	metricReachedSizedLimit            = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_size_limit_has_hit"})
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
	EnableDeleteKeys   bool
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
		cleanUpPerformed, numOfDeletedKeys, numOfKeysToDelete, err := doCleanup(sm.tables, sm.config.TimeLimit, sm.config.SizeLimitBytes, sm.stats, sm.config.DeletionBatchSize, sm.config.GCThreshold, sm.config.EnableDeleteKeys)
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
		glog.V(common.GlogVerbose).Infof("GC finished in %v with error '%v'.  Next run in %v", time.Since(before), err, sm.config.Freq)

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
			glog.V(common.GlogVerbose).Infof("RunValueLogGC(%v) run took %v and returned '%v'", sm.config.BadgerDiscardRatio, time.Since(before), err)
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

func doCleanup(tables typed.Tables, timeLimit time.Duration, sizeLimitBytes int, stats *storeStats, deletionBatchSize int, gcThreshold float64, enableDeletePrefix bool) (bool, int64, int64, error) {
	anyCleanupPerformed := false
	var totalNumOfDeletedKeys int64 = 0
	var totalNumOfKeysToDelete int64 = 0
	partitionsToDelete, partitionsInfoMap := getPartitionsToDelete(tables, timeLimit, sizeLimitBytes, stats.DiskSizeBytes, gcThreshold)

	beforeGCTime := time.Now()
	for _, partitionToDelete := range partitionsToDelete {
		partitionInfo := partitionsInfoMap[partitionToDelete]
		numOfDeletedKeysForPrefix, numOfKeysToDeleteForPrefix, errMessages := deletePartition(partitionToDelete, tables, deletionBatchSize, enableDeletePrefix, partitionInfo)
		anyCleanupPerformed = true
		if len(errMessages) != 0 {
			var errMsg string
			for _, er := range errMessages {
				errMsg += er + ","
			}
			return false, totalNumOfDeletedKeys, totalNumOfKeysToDelete, fmt.Errorf(errMsg)
		}

		glog.V(common.GlogVerbose).Infof("Removed number of keys so far: %v ", totalNumOfDeletedKeys)
		totalNumOfDeletedKeys += int64(numOfDeletedKeysForPrefix)
		totalNumOfKeysToDelete += int64(numOfKeysToDeleteForPrefix)

	}

	elapsed := time.Since(beforeGCTime)
	glog.V(common.GlogVerbose).Infof("Deletion/dropPrefix of prefixes took %v:", elapsed)

	if enableDeletePrefix {
		beforeDropPrefix := time.Now()
		glog.Infof("Deleted %d keys", totalNumOfDeletedKeys)
		// dropping prefix to force compression in case of keys deleted
		err := tables.Db().DropPrefix([]byte{})
		glog.Infof("Drop prefix took %v with error: %v", time.Since(beforeDropPrefix), err)
	}
	return anyCleanupPerformed, totalNumOfDeletedKeys, totalNumOfKeysToDelete, nil
}

func getMinAndMaxPartitionsAndSetMetrics(tables typed.Tables) (bool, string, string) {
	ok, minPartition, maxPartition, err := tables.GetMinAndMaxPartition()
	if err != nil {
		glog.Errorf("failed to get min partition : %s, max partition: %s, err:%v", minPartition, maxPartition, err)
		return false, "", ""
	}

	if !ok {
		return false, "", ""
	}

	minPartitionAge, err := untyped.GetAgeOfPartitionInHours(minPartition)
	if err == nil {
		metricAgeOfMinimumPartition.Set(minPartitionAge)
	}

	maxPartitionAge, err := untyped.GetAgeOfPartitionInHours(maxPartition)
	if err == nil {
		metricAgeOfMaximumPartition.Set(maxPartitionAge)
	}

	return true, minPartition, maxPartition
}

func getPartitionsToDeleteWhenSizeConditionHasBeenMet(sizeLimitBytes int, diskSizeBytes int64, gcThreshold float64, totalKeysCount uint64, partitionMap map[string]*common.PartitionInfo, sortedPartitionsList []string) []string {
	var partitionsToDelete []string
	var keysToBeCollected uint64 = 0
	garbageCollectionRatio := getGarbageCollectionRatio(float64(diskSizeBytes), sizeLimitBytes, gcThreshold)
	numOfKeysToDeleteForFileSizeCondition := getNumberOfKeysToDelete(garbageCollectionRatio, totalKeysCount)
	index := 0
	for keysToBeCollected < numOfKeysToDeleteForFileSizeCondition {
		keysToBeCollected += partitionMap[sortedPartitionsList[index]].TotalKeyCount
		partitionsToDelete = append(partitionsToDelete, sortedPartitionsList[index])
		index++
	}

	return partitionsToDelete
}

func getPartitionsToDelete(tables typed.Tables, timeLimit time.Duration, sizeLimitBytes int, diskSizeBytes int64, gcThreshold float64) ([]string, map[string]*common.PartitionInfo) {

	ok, minPartition, maxPartition := getMinAndMaxPartitionsAndSetMetrics(tables)
	if !ok {
		return nil, nil
	}

	// check if size condition has been met
	sizeConditionMet := hasFilesOnDiskExceededThreshold(diskSizeBytes, sizeLimitBytes, gcThreshold)

	needCleanUp := sizeConditionMet || cleanUpTimeCondition(minPartition, maxPartition, timeLimit)
	if !needCleanUp {
		return nil, nil
	}

	partitionMap, totalKeysCount := common.GetPartitionsInfo(tables.Db())
	var partitionsToDelete []string
	sortedPartitionsList := common.GetSortedPartitionIDs(partitionMap)

	if sizeConditionMet {
		partitionsToDelete = getPartitionsToDeleteWhenSizeConditionHasBeenMet(sizeLimitBytes, diskSizeBytes, gcThreshold, totalKeysCount, partitionMap, sortedPartitionsList)
	}

	// if all the partitions have to be cleaned uo there is no need to further check for time condition
	if len(partitionsToDelete) == len(sortedPartitionsList) {
		return partitionsToDelete, partitionMap
	}

	// Is clean up condition still not met for partitions collected for size
	currentLastPartitionToDeleteIndex := len(partitionsToDelete)
	for currentLastPartitionToDeleteIndex < len(sortedPartitionsList) && cleanUpTimeCondition(sortedPartitionsList[currentLastPartitionToDeleteIndex], maxPartition, timeLimit) {
		partitionsToDelete = append(partitionsToDelete, sortedPartitionsList[currentLastPartitionToDeleteIndex])
		currentLastPartitionToDeleteIndex++
	}

	minPartitionAge, err := untyped.GetAgeOfPartitionInHours(sortedPartitionsList[currentLastPartitionToDeleteIndex])
	if err == nil {
		metricAgeOfMinimumPartition.Set(minPartitionAge)
	}

	return partitionsToDelete, partitionMap
}

func deletePartition(minPartition string, tables typed.Tables, deletionBatchSize int, enableDeleteKeys bool, partitionInfo *common.PartitionInfo) (uint64, uint64, []string) {
	var totalNumOfDeletedKeysForPrefix uint64 = 0
	var totalNumOfKeysToDeleteForPrefix uint64 = 0
	var numOfDeletedKeysForPrefix uint64 = 0
	var numOfKeysToDeleteForPrefix uint64 = 0

	partStart, partEnd, err := untyped.GetTimeRangeForPartition(minPartition)
	glog.Infof("GC removing partition %q with data from %v to %v (err %v)", minPartition, partStart, partEnd, err)
	var errMessages []string
	for _, tableName := range tables.GetTableNames() {
		prefix := fmt.Sprintf("/%s/%s", tableName, minPartition)
		start := time.Now()
		numberOfKeysToRemove := partitionInfo.TableNameToKeyCountMap[tableName]
		if enableDeleteKeys {
			err, numOfDeletedKeysForPrefix, numOfKeysToDeleteForPrefix = common.DeleteKeysWithPrefix(prefix, tables.Db(), deletionBatchSize, numberOfKeysToRemove)
			metricGcDeletedNumberOfKeysByTable.WithLabelValues(fmt.Sprintf("%v", tableName)).Set(float64(numOfDeletedKeysForPrefix))
		} else {

			beforeDropPrefix := time.Now()
			err = tables.Db().DropPrefix([]byte(prefix))

			// !badger!move keys for the given prefix should also be cleaned up. For details: https://github.com/dgraph-io/badger/issues/1288
			err = tables.Db().DropPrefix([]byte("!badger!move" + prefix))

			metricDropPrefixLatency.Set(time.Since(beforeDropPrefix).Seconds())
			// there will be same deletions for dropPrefix as the tables are locked when prefixes are dropped
			numOfDeletedKeysForPrefix = numberOfKeysToRemove
			numOfKeysToDeleteForPrefix = numberOfKeysToRemove
		}

		elapsed := time.Since(start)
		glog.V(common.GlogVerbose).Infof("Call to DropPrefix(%v) took %v and removed %d keys with error: %v", prefix, elapsed, numOfDeletedKeysForPrefix, err)
		if err != nil {
			errMessages = append(errMessages, fmt.Sprintf("failed to cleanup with min key: %s, elapsed: %v,err: %v,", prefix, elapsed, err))
		}

		totalNumOfDeletedKeysForPrefix += numOfDeletedKeysForPrefix
		totalNumOfKeysToDeleteForPrefix += numOfKeysToDeleteForPrefix
	}

	return totalNumOfDeletedKeysForPrefix, totalNumOfKeysToDeleteForPrefix, errMessages
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

func cleanUpFileSizeCondition(stats *storeStats, sizeLimitBytes int, gcThreshold float64, enableDeleteKeys bool, numOfKeysToDeleteForFileSizeCondition int64) bool {
	if enableDeleteKeys {
		return numOfKeysToDeleteForFileSizeCondition > 0
	} else {
		return hasFilesOnDiskExceededThreshold(stats.DiskSizeBytes, sizeLimitBytes, gcThreshold)
	}
}

func hasFilesOnDiskExceededThreshold(diskSizeBytes int64, sizeLimitBytes int, gcThreshold float64) bool {

	// gcThreshold is the threshold when reached would trigger the garbage collection. Its because we want to proactively start GC when the size limit is about to hit.
	sizeThreshold := gcThreshold * float64(sizeLimitBytes)
	currentDiskSize := float64(diskSizeBytes)
	if currentDiskSize > sizeThreshold {
		glog.Infof("Start cleaning up because current file size: %v exceeds file size threshold: %v", diskSizeBytes, sizeThreshold)
		metricReachedSizedLimit.Set(1)
		return true
	}
	glog.V(2).Infof("Can not clean up, disk size: %v is not exceeding size limit: %v yet", diskSizeBytes, uint64(sizeThreshold))
	metricReachedSizedLimit.Set(0)
	return false
}

func getGarbageCollectionRatio(currentDiskSize float64, sizeLimitBytes int, gcThreshold float64) float64 {
	sizeThreshold := gcThreshold * float64(sizeLimitBytes)
	if currentDiskSize > sizeThreshold {

		garbageCollectionRatio := (currentDiskSize - sizeThreshold) / currentDiskSize
		return garbageCollectionRatio
	}
	return 0.0
}

func getNumberOfKeysToDelete(garbageCollectionRatio float64, totalKeyCount uint64) uint64 {
	metricTotalNumberOfKeys.Set(float64(totalKeyCount))

	if garbageCollectionRatio <= 0 || garbageCollectionRatio > 1 {
		// print float here and below
		glog.V(2).Infof("Garbage collection ratio out of bounds. Unexpected ratio: %f", garbageCollectionRatio)
		return 0
	}

	keysToDelete := garbageCollectionRatio * float64(totalKeyCount)
	return uint64(math.Ceil(keysToDelete))
}
