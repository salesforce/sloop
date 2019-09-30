/**
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * Licensed under the BSD 3-Clause license.
 * For full license text, see LICENSE.txt file in the repo root or
 * https://opensource.org/licenses/BSD-3-Clause
 */

package server

import (
	"flag"
	"github.com/salesforce/sloop/pkg/sloop/ingress"
	"github.com/salesforce/sloop/pkg/sloop/server/internal/config"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"os"
	"path"
	"strings"

	"github.com/golang/glog"

	"fmt"
	"github.com/salesforce/sloop/pkg/sloop/processing"
	"github.com/salesforce/sloop/pkg/sloop/queries"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/salesforce/sloop/pkg/sloop/storemanager"
	"github.com/salesforce/sloop/pkg/sloop/webserver"
	"github.com/spf13/afero"
	"net/url"
	"time"
)

const alsologtostderr = "alsologtostderr"

// For easier use in e2e tests
// This is a little ugly and we may want a better solution, but if config says
// to run a single query it returns the output.  When running webserver the output is nil
func RunWithConfig(conf *config.SloopConfig) ([]byte, error) {
	err := conf.Validate()
	if err != nil {
		return []byte{}, err
	}

	kubeClient, kubeContext, err := ingress.MakeKubernetesClient(conf.ApiServerHost, conf.UseKubeContext)
	if err != nil {
		return []byte{}, err
	}

	// Channel used for updates from ingress to store
	// The channel is owned by this function, and no external code should close this!
	kubeWatchChan := make(chan typed.KubeWatchResult, 1000)

	var factory badgerwrap.Factory
	// Setup badger
	if conf.UseMockBadger {
		factory = &badgerwrap.MockFactory{}
	} else {
		factory = &badgerwrap.BadgerFactory{}
	}

	storeRootWithKubeContext := path.Join(conf.StoreRoot, kubeContext)
	db, err := untyped.OpenStore(factory, storeRootWithKubeContext, time.Duration(1)*time.Hour)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to init untyped store: %v", err)
	}
	defer untyped.CloseStore(db)

	tables := typed.NewTableList(db)
	processor := processing.NewProcessing(kubeWatchChan, tables, conf.KeepMinorNodeUpdates, conf.MaxLookback)
	processor.Start()

	// Real kubernetes watcher
	var kubeWatcherSource ingress.KubeWatcher
	if !conf.DisableKubeWatcher {
		kubeWatcherSource, err = ingress.NewKubeWatcherSource(kubeClient, kubeWatchChan, conf.KubeWatchResyncInterval, conf.Crds)
		if err != nil {
			return []byte{}, fmt.Errorf("failed to initialize kubeWatcher: %v", err)
		}
	}

	// File playback
	if conf.DebugPlaybackFile != "" {
		err = ingress.PlayFile(kubeWatchChan, conf.DebugPlaybackFile)
		if err != nil {
			return []byte{}, fmt.Errorf("failed to play back file: %v", err)
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
		storemgr = storemanager.NewStoreManager(tables, conf.StoreRoot, conf.CleanupFrequency, conf.MaxLookback, conf.MaxDiskMb, fs)
		storemgr.Start()
	}

	displayContext := kubeContext
	if conf.DisplayContext != "" {
		displayContext = conf.DisplayContext
	}
	if !conf.DebugDisableWebServer {
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
			return []byte{}, fmt.Errorf("failed to run webserver: %v", err)
		}
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

	if conf.DebugRunQuery != "" {
		params := url.Values(map[string][]string{
			queries.NamespaceParam: {queries.AllNamespaces},
			queries.KindParam:      {queries.AllKinds},
		})
		queryData, err := queries.RunQuery(conf.DebugRunQuery, params, tables, conf.MaxLookback, "server")
		if err != nil {
			return []byte{}, fmt.Errorf("run debug query failed with: %v", err)
		}
		return queryData, nil
	}

	if recorder != nil {
		recorder.Close()
	}

	if storemgr != nil {
		storemgr.Shutdown()
	}

	glog.Infof("RunWithConfig finished")
	return []byte{}, nil
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

func RealMain() error {
	defer glog.Flush()
	setupStdErrLogging()

	config := config.Init() // internally this calls flag.parse
	glog.Infof("SloopConfig: %v", config.ToYaml())

	_, err := RunWithConfig(config)
	return err
}
