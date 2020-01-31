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
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/spf13/afero"
	"sync"
	"time"
)

var (
	metricGcRunCount         = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_gc_run_count"})
	metricGcSuccessCount     = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_gc_success_count"})
	metricGcFailedCount      = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_gc_failed_count"})
	metricGcLatency          = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_latency_sec"})
	metricGcRunning          = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_gc_running"})
	metricValueLogGcRunCount = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_valueloggc_run_count"})
	metricValueLogGcLatency  = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_valueloggc_latency_sec"})
	metricValueLogGcRunning  = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_valueloggc_running"})
)

type Config struct {
	StoreRoot          string
	Freq               time.Duration
	TimeLimit          time.Duration
	SizeLimitBytes     int
	BadgerDiscardRatio float64
	BadgerVLogGCFreq   time.Duration
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
		_, err := doCleanup(sm.tables, sm.config.TimeLimit, sm.config.SizeLimitBytes, sm.stats)
		metricGcRunning.Set(0)
		if err == nil {
			metricGcSuccessCount.Inc()
		} else {
			metricGcFailedCount.Inc()
		}
		metricGcLatency.Set(time.Since(before).Seconds())
		glog.Infof("GC finished in %v with return %q.  Next run in %v", time.Since(before), err, sm.config.Freq)

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

func doCleanup(tables typed.Tables, timeLimit time.Duration, sizeLimitBytes int, stats *storeStats) (bool, error) {
	ok, minPartition, maxPartition, err := tables.GetMinAndMaxPartition()
	if err != nil {
		return false, fmt.Errorf("failed to get min partition : %s, max partition: %s, err:%v", minPartition, maxPartition, err)
	}
	if !ok {
		return false, nil
	}

	anyCleanupPerformed := false
	if cleanUpTimeCondition(minPartition, maxPartition, timeLimit) || cleanUpFileSizeCondition(stats, sizeLimitBytes) {
		partStart, partEnd, err := untyped.GetTimeRangeForPartition(minPartition)
		glog.Infof("GC removing partition %q with data from %v to %v (err %v)", minPartition, partStart, partEnd, err)
		var errMsgs []string
		for _, tableName := range tables.GetTableNames() {
			prefix := fmt.Sprintf("/%s/%s", tableName, minPartition)
			start := time.Now()
			err = tables.Db().DropPrefix([]byte(prefix))
			elapsed := time.Since(start)
			glog.Infof("Call to DropPrefix(%v) took %v and returned %v", prefix, elapsed, err)
			if err != nil {
				errMsgs = append(errMsgs, fmt.Sprintf("failed to cleanup with min key: %s, elapsed: %v,err: %v,", prefix, elapsed, err))
			}
			anyCleanupPerformed = true
		}

		if len(errMsgs) != 0 {
			var errMsg string
			for _, er := range errMsgs {
				errMsg += er + ","
			}
			return false, fmt.Errorf(errMsg)
		}
	}

	return anyCleanupPerformed, nil
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

func cleanUpFileSizeCondition(stats *storeStats, sizeLimitBytes int) bool {
	if stats.DiskSizeBytes > int64(sizeLimitBytes) {
		glog.Infof("Start cleaning up because current file size: %v exceeds file size: %v", stats.DiskSizeBytes, sizeLimitBytes)
		return true
	}
	glog.V(2).Infof("Can not clean up, disk size: %v is not exceeding size limit: %v yet", stats.DiskSizeBytes, uint64(sizeLimitBytes))
	return false
}
