/**
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package storemanager

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/spf13/afero"
	"os"
	"sync"
	"time"
)

var (
	metricGcRunCount              = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_gc_run_count"})
	metricGcCleanupPerformedCount = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_gc_cleanup_performed_count"})
	metricGcFailedCount           = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_failed_gc_count"})
	metricStoreSizeondiskmb       = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_store_sizeondiskmb"})
	metricBadgerKeys              = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_badger_keys"})
	metricBadgerTables            = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_badger_tables"})
	metricBadgerLsmsizemb         = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_badger_lsmsizemb"})
	metricBadgerVlogsizemb        = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_badger_vlogsizemb"})
)

type StoreManager struct {
	tables      typed.Tables
	storeRoot   string
	freq        time.Duration
	timeLimit   time.Duration
	sizeLimitMb int
	fs          *afero.Afero
	testMode    bool
	sleeper     *SleepWithCancel
	wg          *sync.WaitGroup
	done        bool
	donelock    *sync.Mutex
}

func NewStoreManager(tables typed.Tables, storeRoot string, freq time.Duration, timeLimit time.Duration, sizeLimitMb int, fs *afero.Afero) *StoreManager {
	return &StoreManager{
		tables:      tables,
		storeRoot:   storeRoot,
		freq:        freq,
		timeLimit:   timeLimit,
		sizeLimitMb: sizeLimitMb,
		fs:          fs,
		sleeper:     NewSleepWithCancel(),
		wg:          &sync.WaitGroup{},
		done:        false,
		donelock:    &sync.Mutex{},
	}
}

func (sm *StoreManager) isDone() bool {
	sm.donelock.Lock()
	defer sm.donelock.Unlock()
	return sm.done
}

func (sm *StoreManager) Start() {
	go func() {
		sm.wg.Add(1)
		defer sm.wg.Done()
		for {
			if sm.isDone() {
				glog.Infof("Store manager main loop exiting")
				return
			}
			temporaryEmitMetrics(sm.storeRoot, sm.tables.Db(), sm.fs)
			metricGcRunCount.Inc()
			cleanupPerformed, err := doCleanup(sm.tables, sm.storeRoot, sm.timeLimit, sm.sizeLimitMb*1024*1024, sm.fs)
			if err != nil {
				glog.Errorf("GC failed with err:%v, will sleep: %v and retry later ...", err, sm.freq)
				sm.sleeper.Sleep(sm.freq)
			} else if !cleanupPerformed {
				glog.V(2).Infof("GC did not need to clean anything, will sleep: %v", sm.freq)
				sm.sleeper.Sleep(sm.freq)
			} else {
				// We did some cleanup and there were no errors.  Because we may be in a low space situation lets skip
				// the sleep and repeat the loop
				glog.Infof("GC cleanup performed")
				metricGcCleanupPerformedCount.Inc()
			}
		}
	}()
}

func (sm *StoreManager) Shutdown() {
	glog.Infof("Starting store manager shutdown")
	sm.donelock.Lock()
	sm.done = true
	sm.donelock.Unlock()
	sm.sleeper.Cancel()
	sm.wg.Wait()
}

func doCleanup(tables typed.Tables, storeRoot string, timeLimit time.Duration, sizeLimitBytes int, fs *afero.Afero) (bool, error) {
	ok, minPartition, maxPartiton, err := tables.GetMinAndMaxPartition()
	if err != nil {
		return false, fmt.Errorf("failed to get min partition : %s, max partition: %s, err:%v", minPartition, maxPartiton, err)
	}
	if !ok {
		return false, nil
	}

	anyCleanupPerformed := false
	if cleanUpTimeCondition(minPartition, maxPartiton, timeLimit) || cleanUpFileSizeCondition(storeRoot, sizeLimitBytes, fs) {
		var errMsgs []string
		for _, tableName := range tables.GetTableNames() {
			prefix := fmt.Sprintf("/%s/%s", tableName, minPartition)
			start := time.Now()
			err = tables.Db().DropPrefix([]byte(prefix))
			elapsed := time.Since(start)
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

func cleanUpFileSizeCondition(storeRoot string, sizeLimitBytes int, fs *afero.Afero) bool {
	size, err := getDirSizeRecursive(storeRoot, fs)
	if err != nil {
		return false
	}

	if size > uint64(sizeLimitBytes) {
		glog.Infof("Start cleaning up because current file size: %v exceeds file size: %v", size, sizeLimitBytes)
		return true
	}
	glog.V(2).Infof("Can not clean up, disk size: %v is not exceeding size limit: %v yet", size, uint64(sizeLimitBytes))
	return false
}

func getDirSizeRecursive(root string, fs *afero.Afero) (uint64, error) {
	var totalSize uint64

	err := fs.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			totalSize += uint64(info.Size())
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return totalSize, nil
}

// TODO: Properly integrate this with the next refactor
func temporaryEmitMetrics(storeRoot string, db badgerwrap.DB, fs *afero.Afero) {
	totalSizeBytes, err := getDirSizeRecursive(storeRoot, fs)
	if err != nil {
		glog.Errorf("Failed to check storage size on disk")
	} else {
		metricStoreSizeondiskmb.Set(float64(totalSizeBytes) / 1024.0 / 1024.0)
	}

	lsmSize, vlogSize := db.Size()
	metricBadgerLsmsizemb.Set(float64(lsmSize) / 1024.0 / 1024.0)
	metricBadgerVlogsizemb.Set(float64(vlogSize) / 1024.0 / 1024.0)

	var totalKeys uint64
	tables := db.Tables(true)
	for _, table := range tables {
		totalKeys += table.KeyCount
	}
	metricBadgerKeys.Set(float64(totalKeys))
	metricBadgerTables.Set(float64(len(tables)))
}
