/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package server

import (
	"flag"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/salesforce/sloop/pkg/sloop/ingress"
	"github.com/salesforce/sloop/pkg/sloop/server/internal/config"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"

	"github.com/golang/glog"

	"github.com/spf13/afero"

	"github.com/salesforce/sloop/pkg/sloop/processing"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/salesforce/sloop/pkg/sloop/storemanager"
	"github.com/salesforce/sloop/pkg/sloop/webserver"
)

const alsologtostderr = "alsologtostderr"

func RealMain() error {
	defer glog.Flush()
	setupStdErrLogging()

	conf := config.Init() // internally this calls flag.parse
	glog.Infof("SloopConfig: %v", conf.ToYaml())

	err := conf.Validate()
	if err != nil {
		return errors.Wrap(err, "config validation failed")
	}

	kubeContext, err := ingress.GetKubernetesContext(conf.ApiServerHost, conf.UseKubeContext)
	if err != nil {
		return errors.Wrap(err, "failed to get kubernetes context")
	}

	// Channel used for updates from ingress to store
	// The channel is owned by this function, and no external code should close this!
	kubeWatchChan := make(chan typed.KubeWatchResult, 1000)

	factory := &badgerwrap.BadgerFactory{}

	storeRootWithKubeContext := path.Join(conf.StoreRoot, kubeContext)
	storeConfig := &untyped.Config{
		RootPath:                 storeRootWithKubeContext,
		ConfigPartitionDuration:  time.Duration(1) * time.Hour,
		BadgerMaxTableSize:       conf.BadgerMaxTableSize,
		BadgerKeepL0InMemory:     conf.BadgerKeepL0InMemory,
		BadgerVLogFileSize:       conf.BadgerVLogFileSize,
		BadgerVLogMaxEntries:     conf.BadgerVLogMaxEntries,
		BadgerUseLSMOnlyOptions:  conf.BadgerUseLSMOnlyOptions,
		BadgerEnableEventLogging: conf.BadgerEnableEventLogging,
		BadgerNumOfCompactors:    conf.BadgerNumOfCompactors,
		BadgerNumL0Tables:        conf.BadgerNumL0Tables,
		BadgerNumL0TablesStall:   conf.BadgerNumL0TablesStall,
		BadgerSyncWrites:         conf.BadgerSyncWrites,
		BadgerLevelOneSize:       conf.BadgerLevelOneSize,
		BadgerLevSizeMultiplier:  conf.BadgerLevSizeMultiplier,
	}
	db, err := untyped.OpenStore(factory, storeConfig)
	if err != nil {
		return errors.Wrap(err, "failed to init untyped store")
	}
	defer untyped.CloseStore(db)

	if conf.RestoreDatabaseFile != "" {
		glog.Infof("Restoring from backup file %q into context %q", conf.RestoreDatabaseFile, kubeContext)
		err := ingress.DatabaseRestore(db, conf.RestoreDatabaseFile)
		if err != nil {
			return errors.Wrap(err, "failed to restore database")
		}
		glog.Infof("Restored from backup file %q into context %q", conf.RestoreDatabaseFile, kubeContext)
	}

	tables := typed.NewTableList(db)
	processor := processing.NewProcessing(kubeWatchChan, tables, conf.KeepMinorNodeUpdates, conf.MaxLookback)
	processor.Start()

	// Real kubernetes watcher
	var kubeWatcherSource ingress.KubeWatcher
	if !conf.DisableKubeWatcher {
		kubeClient, err := ingress.MakeKubernetesClient(conf.ApiServerHost, kubeContext)
		if err != nil {
			return errors.Wrap(err, "failed to create kubernetes client")
		}

		kubeWatcherSource, err = ingress.NewKubeWatcherSource(kubeClient, kubeWatchChan, conf.KubeWatchResyncInterval, conf.WatchCrds, conf.ApiServerHost, kubeContext)
		if err != nil {
			return errors.Wrap(err, "failed to initialize kubeWatcher")
		}
	}

	// File playback
	if conf.DebugPlaybackFile != "" {
		err = ingress.PlayFile(kubeWatchChan, conf.DebugPlaybackFile)
		if err != nil {
			return errors.Wrap(err, "failed to play back file")
		}
	}

	var recorder *ingress.FileRecorder
	if conf.DebugRecordFile != "" {
		recorder = ingress.NewFileRecorder(conf.DebugRecordFile, kubeWatchChan)
		recorder.Start()
	}

	var storemgr *storemanager.StoreManager
	if !conf.DisableStoreManager {
		fs := &afero.Afero{Fs: afero.NewOsFs()}
		storeCfg := &storemanager.Config{
			StoreRoot:          conf.StoreRoot,
			Freq:               conf.CleanupFrequency,
			TimeLimit:          conf.MaxLookback,
			SizeLimitBytes:     conf.MaxDiskMb * 1024 * 1024,
			BadgerDiscardRatio: conf.BadgerDiscardRatio,
			BadgerVLogGCFreq:   conf.BadgerVLogGCFreq,
			DeletionBatchSize:  conf.DeletionBatchSize,
			GCThreshold:        conf.ThresholdForGC,
		}
		storemgr = storemanager.NewStoreManager(tables, storeCfg, fs)
		storemgr.Start()
	}

	displayContext := kubeContext
	if conf.DisplayContext != "" {
		displayContext = conf.DisplayContext
	}

	webConfig := webserver.WebConfig{
		Port:             conf.Port,
		WebFilesPath:     conf.WebFilesPath,
		ConfigYaml:       conf.ToYaml(),
		MaxLookback:      conf.MaxLookback,
		DefaultNamespace: conf.DefaultNamespace,
		DefaultLookback:  conf.DefaultLookback,
		DefaultResources: conf.DefaultKind,
		ResourceLinks:    conf.ResourceLinks,
		LeftBarLinks:     conf.LeftBarLinks,
		CurrentContext:   displayContext,
	}
	err = webserver.Run(webConfig, tables)
	if err != nil {
		return errors.Wrap(err, "failed to run webserver")
	}

	// Initiate shutdown with the following order:
	// 1. Shut down ingress so that it stops emitting events
	// 2. Close the input channel which signals processing to finish work
	// 3. Wait on processor to tell us all work is complete.  Store will not change after that
	if kubeWatcherSource != nil {
		kubeWatcherSource.Stop()
	}
	close(kubeWatchChan)
	processor.Wait()

	if recorder != nil {
		recorder.Close()
	}

	if storemgr != nil {
		storemgr.Shutdown()
	}

	glog.Infof("RunWithConfig finished")
	return nil
}

// By default glog will not print anything to console, which can confuse users
// This will turn it on unless user sets it explicitly (with --alsologtostderr=false)
func setupStdErrLogging() {
	for _, arg := range os.Args[1:] {
		if strings.Contains(arg, alsologtostderr) {
			return
		}
	}
	err := flag.Set("alsologtostderr", "true")
	if err != nil {
		panic(err)
	}
}
