/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package processing

import (
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/salesforce/sloop/pkg/sloop/kubeextractor"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"sync"
	"time"
)

type Runner struct {
	kubeWatchChan        chan typed.KubeWatchResult
	tables               typed.Tables
	inputWg              *sync.WaitGroup
	keepMinorNodeUpdates bool
	maxLookback          time.Duration
}

var (
	metricProcessingWatchtableUpdatecount = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_processing_watchtable_updatecount"})
	metricIngestionFailureCount           = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_ingestion_failure_count"})
	metricIngestionSuccessCount           = promauto.NewCounter(prometheus.CounterOpts{Name: "sloop_ingestion_success_count"})
)

func NewProcessing(kubeWatchChan chan typed.KubeWatchResult, tables typed.Tables, keepMinorNodeUpdates bool, maxLookback time.Duration) *Runner {
	return &Runner{kubeWatchChan: kubeWatchChan, tables: tables, inputWg: &sync.WaitGroup{}, keepMinorNodeUpdates: keepMinorNodeUpdates, maxLookback: maxLookback}
}

func (r *Runner) processingFailed(name string, err error) {
	glog.Errorf("Processing for %v failed with error %v", name, err)
	metricIngestionFailureCount.Inc()
}

func (r *Runner) Start() {
	r.inputWg.Add(1)
	go func() {
		for {
			watchRec, more := <-r.kubeWatchChan
			if !more {
				r.inputWg.Done()
				return
			}

			resourceMetadata, err := kubeextractor.ExtractMetadata(watchRec.Payload)
			if err != nil {
				r.processingFailed("cannot extract resource metadata", err)
			}
			involvedObject, err := kubeextractor.ExtractInvolvedObject(watchRec.Payload)
			if err != nil {
				r.processingFailed("cannot extract involved object", err)
			}

			// Processing event count first so it can easily find the previous copy of the event
			// If we update watchTable first then this will see the new event and think it is a dupe
			err = r.tables.Db().Update(func(txn badgerwrap.Txn) error {
				return updateEventCountTable(r.tables, txn, &watchRec, &resourceMetadata, &involvedObject, r.maxLookback)
			})
			if err != nil {
				r.processingFailed("updateEventCountTable", err)
			}

			err = r.tables.Db().Update(func(txn badgerwrap.Txn) error {
				return updateWatchActivityTable(r.tables, txn, &watchRec, &resourceMetadata)
			})
			if err != nil {
				r.processingFailed("updateWatchActivityTable", err)
			}

			err = r.tables.Db().Update(func(txn badgerwrap.Txn) error {
				return updateKubeWatchTable(r.tables, txn, &watchRec, &resourceMetadata, r.keepMinorNodeUpdates)
			})
			if err != nil {
				r.processingFailed("updateKubeWatchTable", err)
			}

			err = r.tables.Db().Update(func(txn badgerwrap.Txn) error {
				return updateResourceSummaryTable(r.tables, txn, &watchRec, &resourceMetadata)
			})
			if err != nil {
				r.processingFailed("updateResourceSummaryTable", err)
			}
		}
	}()
}

func (r *Runner) Wait() {
	glog.Infof("Waiting for outstanding processing to finish")
	r.inputWg.Wait()
}
