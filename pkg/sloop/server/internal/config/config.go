/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package config

import (
	"flag"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/salesforce/sloop/pkg/sloop/webserver"
	"io/ioutil"
	"os"
	"time"
)

const sloopConfigEnvVar = "SLOOP_CONFIG"

type SloopConfig struct {
	// These fields can only come from command line
	ConfigFile string
	// These fields can only come from file because they use complex types
	LeftBarLinks  []webserver.LinkTemplate         `json:"leftBarLinks"`
	ResourceLinks []webserver.ResourceLinkTemplate `json:"resourceLinks"`
	// Normal fields that can come from file or cmd line
	DisableKubeWatcher      bool          `json:"disableKubeWatch"`
	KubeWatchResyncInterval time.Duration `json:"kubeWatchResyncInterval"`
	WebFilesPath            string        `json:"webfilesPath"`
	Port                    int           `json:"port"`
	StoreRoot               string        `json:"storeRoot"`
	MaxLookback             time.Duration `json:"maxLookBack"`
	MaxDiskMb               int           `json:"maxDiskMb"`
	DebugPlaybackFile       string        `json:"debugPlaybackFile"`
	DebugRecordFile         string        `json:"debugRecordFile"`
	UseMockBadger           bool          `json:"mockBadger"`
	DisableStoreManager     bool          `json:"disableStoreManager"`
	CleanupFrequency        time.Duration `json:"cleanupFrequency" validate:"min=1h,max=120h"`
	KeepMinorNodeUpdates    bool          `json:"keepMinorNodeUpdates"`
	DefaultNamespace        string        `json:"defaultNamespace"`
	DefaultKind             string        `json:"defaultKind"`
	DefaultLookback         string        `json:"defaultLookback"`
	UseKubeContext          string        `json:"context"`
	DisplayContext          string        `json:"displayContext"`
	ApiServerHost           string        `json:"apiServerHost"`
	WatchCrds               bool          `json:"watchCrds"`
}

func registerFlags(fs *flag.FlagSet, config *SloopConfig) {
	fs.StringVar(&config.ConfigFile, "config", "", "Path to a yaml or json config file")
	fs.BoolVar(&config.DisableKubeWatcher, "disable-kube-watch", false, "Turn off kubernetes watch")
	fs.DurationVar(&config.KubeWatchResyncInterval, "kube-watch-resync-interval", 30*time.Minute,
		"OPTIONAL: Kubernetes watch resync interval")
	fs.StringVar(&config.WebFilesPath, "web-files-path", "./pkg/sloop/webfiles", "Path to web files")
	fs.IntVar(&config.Port, "port", 8080, "Web server port")
	fs.StringVar(&config.StoreRoot, "store-root", "./data", "Path to store history data")
	fs.DurationVar(&config.MaxLookback, "max-look-back", time.Duration(14*24)*time.Hour, "Max history data to keep")
	fs.IntVar(&config.MaxDiskMb, "max-disk-mb", 32*1024, "Max disk storage in MB")
	fs.StringVar(&config.DebugPlaybackFile, "playback-file", "", "Read watch data from a playback file")
	fs.StringVar(&config.DebugRecordFile, "record-file", "", "Record watch data to a playback file")
	fs.BoolVar(&config.UseMockBadger, "use-mock-badger", false, "Use a fake in-memory mock of badger")
	fs.BoolVar(&config.DisableStoreManager, "disable-store-manager", false, "Turn off store manager which is to clean up database")
	fs.DurationVar(&config.CleanupFrequency, "cleanup-frequency", time.Minute,
		"OPTIONAL: Frequency between subsequent runs for the database cleanup")
	fs.BoolVar(&config.KeepMinorNodeUpdates, "keep-minor-node-updates", false, "Keep all node updates even if change is only condition timestamps")
	fs.StringVar(&config.UseKubeContext, "context", "", "Use a specific kubernetes context")
	fs.StringVar(&config.DisplayContext, "display-context", "", "Use this to override the display context.  When running in k8s the context is empty string.  This lets you override that (mainly useful if you are running many copies of sloop on different clusters) ")
	fs.StringVar(&config.ApiServerHost, "apiserver-host", "", "Kubernetes API server endpoint")
	fs.BoolVar(&config.WatchCrds, "watch-crds", false, "Watch for activity for CRDs")
}

// This will first check if a config file is specified on cmd line using a temporary flagSet
// If not there, check the environment variable
// If we have a config path, load initial values from it
// Next parse flags again and override any fields from command line
//
// We do this to support settings that can come from either cmd line or config file
func Init() *SloopConfig {
	newConfig := &SloopConfig{}

	configFilename := preParseConfigFlag()
	glog.Infof("Config flag: %s", configFilename)
	if configFilename == "" {
		configFilename = os.Getenv(sloopConfigEnvVar)
		glog.Infof("Config env: %s", configFilename)
	}
	if configFilename != "" {
		newConfig = loadFromFile(configFilename)
	}

	registerFlags(flag.CommandLine, newConfig)
	flag.Parse()
	// Set this to the correct value in case we got it from envVar
	newConfig.ConfigFile = configFilename
	return newConfig
}

func (c *SloopConfig) ToYaml() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (c *SloopConfig) Validate() error {
	if c.MaxLookback <= 0 {
		return fmt.Errorf("SloopConfig value MaxLookback can not be <= 0")
	}
	return nil
}

func loadFromFile(filename string) *SloopConfig {
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("failed to read %v. %v", filename, err))
	}
	var config SloopConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal %v. %v", filename, err))
	}
	return &config
}

// Pre-parse flags and return config filename without side-effects
func preParseConfigFlag() string {
	tempCfg := &SloopConfig{}
	fs := flag.NewFlagSet("configFileOnly", flag.ContinueOnError)
	registerFlags(fs, tempCfg)
	registerDummyGlogFlags(fs)
	err := fs.Parse(os.Args[1:])
	if err != nil {
		fmt.Printf("Failed to pre-parse flags looking for config file: %v\n", err)
	}
	return tempCfg.ConfigFile
}

// The gflags library registers flags in init() in github.com/golang/glog.go but only using the global flag set
// We need to also register them in our temporary flagset so we dont get an error about "flag provided but not
// defined".  We dont care what the values are.
func registerDummyGlogFlags(fs *flag.FlagSet) {
	fs.Bool("logtostderr", false, "log to standard error instead of files")
	fs.Bool("alsologtostderr", false, "log to standard error as well as files")
	fs.Int("v", 0, "log level for V logs")
	fs.Int("stderrthreshold", 0, "logs at or above this threshold go to stderr")
	fs.String("vmodule", "", "comma-separated list of pattern=N settings for file-filtered logging")
	fs.String("log_backtrace_at", "", "when logging hits line file:N, emit a stack trace")
}
