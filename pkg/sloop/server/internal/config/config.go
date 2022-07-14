/*
 * Copyright (c) 2019, salesforce.com, inc.
 * All rights reserved.
 * SPDX-License-Identifier: BSD-3-Clause
 * For full license text, see LICENSE.txt file in the repo root or https://opensource.org/licenses/BSD-3-Clause
 */

package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/salesforce/sloop/pkg/sloop/webserver"
)

const sloopConfigEnvVar = "SLOOP_CONFIG"

type SloopConfig struct {
	// These fields can only come from command line
	ConfigFile string
	// These fields can only come from file because they use complex types
	LeftBarLinks  []webserver.LinkTemplate         `json:"leftBarLinks"`
	ResourceLinks []webserver.ResourceLinkTemplate `json:"resourceLinks"`
	// Normal fields that can come from file or cmd line
	DisableKubeWatcher       bool          `json:"disableKubeWatch"`
	KubeWatchResyncInterval  time.Duration `json:"kubeWatchResyncInterval"`
	WebFilesPath             string        `json:"webfilesPath"`
	BindAddress              string        `json:"bindAddress"`
	Port                     int           `json:"port"`
	StoreRoot                string        `json:"storeRoot"`
	MaxLookback              time.Duration `json:"maxLookBack"`
	MaxDiskMb                int           `json:"maxDiskMb"`
	DebugPlaybackFile        string        `json:"debugPlaybackFile"`
	DebugRecordFile          string        `json:"debugRecordFile"`
	DeletionBatchSize        int           `json:"deletionBatchSize"`
	UseMockBadger            bool          `json:"mockBadger"`
	DisableStoreManager      bool          `json:"disableStoreManager"`
	CleanupFrequency         time.Duration `json:"cleanupFrequency" validate:"min=1h,max=120h"`
	KeepMinorNodeUpdates     bool          `json:"keepMinorNodeUpdates"`
	DefaultNamespace         string        `json:"defaultNamespace"`
	DefaultKind              string        `json:"defaultKind"`
	DefaultLookback          string        `json:"defaultLookback"`
	UseKubeContext           string        `json:"context"`
	DisplayContext           string        `json:"displayContext"`
	ApiServerHost            string        `json:"apiServerHost"`
	WatchCrds                bool          `json:"watchCrds"`
	CrdRefreshInterval       time.Duration `json:"crdRefreshInterval"`
	ThresholdForGC           float64       `json:"threshold for GC"`
	RestoreDatabaseFile      string        `json:"restoreDatabaseFile"`
	BadgerDiscardRatio       float64       `json:"badgerDiscardRatio"`
	BadgerVLogGCFreq         time.Duration `json:"badgerVLogGCFreq"`
	BadgerMaxTableSize       int64         `json:"badgerMaxTableSize"`
	BadgerLevelOneSize       int64         `json:"badgerLevelOneSize"`
	BadgerLevSizeMultiplier  int           `json:"badgerLevSizeMultiplier"`
	BadgerKeepL0InMemory     bool          `json:"badgerKeepL0InMemory"`
	BadgerVLogFileSize       int64         `json:"badgerVLogFileSize"`
	BadgerVLogMaxEntries     uint          `json:"badgerVLogMaxEntries"`
	BadgerUseLSMOnlyOptions  bool          `json:"badgerUseLSMOnlyOptions"`
	BadgerEnableEventLogging bool          `json:"badgerEnableEventLogging"`
	BadgerNumOfCompactors    int           `json:"badgerNumOfCompactors"`
	BadgerNumL0Tables        int           `json:"badgerNumLevelZeroTables"`
	BadgerNumL0TablesStall   int           `json:"badgerNumLevelZeroTablesStall"`
	BadgerSyncWrites         bool          `json:"badgerSyncWrites"`
	BadgerVLogFileIOMapping  bool          `json:"badgerVLogFileIOMapping"`
	BadgerVLogTruncate       bool          `json:"badgerVLogTruncate"`
	EnableDeleteKeys         bool          `json:"enableDeleteKeys"`
	EnableGranularMetrics    bool          `json:"enableGranularMetrics"`
	BadgerDetailLogEnabled   bool          `json:"badgerDetailLogEnabled"`
}

func registerFlags(fs *flag.FlagSet, config *SloopConfig) {
	fs.StringVar(&config.ConfigFile, "config", config.ConfigFile, "Path to a yaml or json config file")
	fs.BoolVar(&config.DisableKubeWatcher, "disable-kube-watch", config.DisableKubeWatcher, "Turn off kubernetes watch")
	fs.DurationVar(&config.KubeWatchResyncInterval, "kube-watch-resync-interval", config.KubeWatchResyncInterval,
		"OPTIONAL: Kubernetes watch resync interval")
	fs.StringVar(&config.WebFilesPath, "web-files-path", config.WebFilesPath, "Path to web files")
	fs.StringVar(&config.BindAddress, "bind-address", config.BindAddress, "Web server bind ip address.")
	fs.IntVar(&config.Port, "port", config.Port, "Web server port")
	fs.StringVar(&config.StoreRoot, "store-root", config.StoreRoot, "Path to store history data")
	fs.DurationVar(&config.MaxLookback, "max-look-back", config.MaxLookback, "Max history data to keep")
	fs.IntVar(&config.MaxDiskMb, "max-disk-mb", config.MaxDiskMb, "Max disk storage in MB")
	fs.StringVar(&config.DebugPlaybackFile, "playback-file", config.DebugPlaybackFile, "Read watch data from a playback file")
	fs.StringVar(&config.DebugRecordFile, "record-file", config.DebugRecordFile, "Record watch data to a playback file")
	fs.BoolVar(&config.UseMockBadger, "use-mock-badger", config.UseMockBadger, "Use a fake in-memory mock of badger")
	fs.BoolVar(&config.DisableStoreManager, "disable-store-manager", config.DisableStoreManager, "Turn off store manager which is to clean up database")
	fs.DurationVar(&config.CleanupFrequency, "cleanup-frequency", config.CleanupFrequency, "Frequency between subsequent runs for the database cleanup")
	fs.BoolVar(&config.KeepMinorNodeUpdates, "keep-minor-node-updates", config.KeepMinorNodeUpdates, "Keep all node updates even if change is only condition timestamps")
	fs.StringVar(&config.DefaultLookback, "default-lookback", config.DefaultLookback, "Default UX filter lookback")
	fs.StringVar(&config.DefaultKind, "default-kind", config.DefaultKind, "Default UX filter kind")
	fs.StringVar(&config.DefaultNamespace, "default-namespace", config.DefaultNamespace, "Default UX filter namespace")
	fs.IntVar(&config.DeletionBatchSize, "deletion-batch-size", config.DeletionBatchSize, "Size of batch for deletion")
	fs.StringVar(&config.UseKubeContext, "context", config.UseKubeContext, "Use a specific kubernetes context")
	fs.StringVar(&config.DisplayContext, "display-context", config.DisplayContext, "Use this to override the display context.  When running in k8s the context is empty string.  This lets you override that (mainly useful if you are running many copies of sloop on different clusters) ")
	fs.StringVar(&config.ApiServerHost, "apiserver-host", config.ApiServerHost, "Kubernetes API server endpoint")
	fs.BoolVar(&config.WatchCrds, "watch-crds", config.WatchCrds, "Watch for activity for CRDs")
	fs.DurationVar(&config.CrdRefreshInterval, "crd-refresh-interval", config.CrdRefreshInterval, "Frequency between CRD Informer refresh")
	fs.StringVar(&config.RestoreDatabaseFile, "restore-database-file", config.RestoreDatabaseFile, "Restore database from backup file into current context.")
	fs.Float64Var(&config.BadgerDiscardRatio, "badger-discard-ratio", config.BadgerDiscardRatio, "Badger value log GC uses this value to decide if it wants to compact a vlog file. The lower the value of discardRatio the higher the number of !badger!move keys. And thus more the number of !badger!move keys, the size on disk keeps on increasing over time.")
	fs.Float64Var(&config.ThresholdForGC, "gc-threshold", config.ThresholdForGC, "Threshold for GC to start garbage collecting")
	fs.DurationVar(&config.BadgerVLogGCFreq, "badger-vlog-gc-freq", config.BadgerVLogGCFreq, "Frequency of running badger's ValueLogGC")
	fs.Int64Var(&config.BadgerMaxTableSize, "badger-max-table-size", config.BadgerMaxTableSize, "Max LSM table size in bytes.  0 = use badger default")
	fs.Int64Var(&config.BadgerLevelOneSize, "badger-level-one-size", config.BadgerLevelOneSize, "The maximum total size for Level 1.  0 = use badger default")
	fs.IntVar(&config.BadgerLevSizeMultiplier, "badger-level-size-multiplier", config.BadgerLevSizeMultiplier, "The ratio between the maximum sizes of contiguous levels in the LSM.  0 = use badger default")
	fs.BoolVar(&config.BadgerKeepL0InMemory, "badger-keep-l0-in-memory", config.BadgerKeepL0InMemory, "Keeps all level 0 tables in memory for faster writes and compactions")
	fs.Int64Var(&config.BadgerVLogFileSize, "badger-vlog-file-size", config.BadgerVLogFileSize, "Max size in bytes per value log file. 0 = use badger default")
	fs.UintVar(&config.BadgerVLogMaxEntries, "badger-vlog-max-entries", config.BadgerVLogMaxEntries, "Max number of entries per value log files. 0 = use badger default")
	fs.BoolVar(&config.BadgerUseLSMOnlyOptions, "badger-use-lsm-only-options", config.BadgerUseLSMOnlyOptions, "Sets a higher valueThreshold so values would be collocated with LSM tree reducing vlog disk usage")
	fs.BoolVar(&config.BadgerEnableEventLogging, "badger-enable-event-logging", config.BadgerEnableEventLogging, "Turns on badger event logging")
	fs.IntVar(&config.BadgerNumOfCompactors, "badger-number-of-compactors", config.BadgerNumOfCompactors, "Number of compactors for badger")
	fs.IntVar(&config.BadgerNumL0Tables, "badger-number-of-level-zero-tables", config.BadgerNumL0Tables, "Number of level zero tables for badger")
	fs.IntVar(&config.BadgerNumL0TablesStall, "badger-number-of-zero-tables-stall", config.BadgerNumL0TablesStall, "Number of Level 0 tables that once reached causes the DB to stall until compaction succeeds")
	fs.BoolVar(&config.BadgerSyncWrites, "badger-sync-writes", config.BadgerSyncWrites, "Sync Writes ensures writes are synced to disk if set to true")
	fs.BoolVar(&config.EnableDeleteKeys, "enable-delete-keys", config.EnableDeleteKeys, "Use delete prefixes instead of dropPrefix for GC")
	fs.BoolVar(&config.BadgerVLogFileIOMapping, "badger-vlog-fileIO-mapping", config.BadgerVLogFileIOMapping, "Indicates which file loading mode should be used for the value log data, in memory constrained environments the value is recommended to be true")
	fs.BoolVar(&config.BadgerVLogTruncate, "badger-vlog-truncate", config.BadgerVLogTruncate, "Truncate value log if badger db offset is different from badger db size")
	fs.BoolVar(&config.BadgerDetailLogEnabled, "badger-detail-log-enabled", config.BadgerDetailLogEnabled, "Turns on detailed logging of BadgerDB")
}

func getDefaultConfig() *SloopConfig {
	defaultConfig := SloopConfig{
		ConfigFile:               "",
		DisableKubeWatcher:       false,
		KubeWatchResyncInterval:  30 * time.Minute,
		WebFilesPath:             "./pkg/sloop/webserver/webfiles",
		BindAddress:              "",
		Port:                     8080,
		StoreRoot:                "./data",
		MaxLookback:              time.Duration(14*24) * time.Hour,
		MaxDiskMb:                32 * 1024,
		DebugPlaybackFile:        "",
		DebugRecordFile:          "",
		DeletionBatchSize:        1000,
		UseMockBadger:            false,
		DisableStoreManager:      false,
		CleanupFrequency:         time.Minute * 30,
		KeepMinorNodeUpdates:     false,
		DefaultNamespace:         "default",
		DefaultKind:              "_all",
		DefaultLookback:          "1h",
		UseKubeContext:           "",
		DisplayContext:           "",
		ApiServerHost:            "",
		WatchCrds:                true,
		CrdRefreshInterval:       time.Duration(5 * time.Minute),
		ThresholdForGC:           0.8,
		RestoreDatabaseFile:      "",
		BadgerDiscardRatio:       0.99,
		BadgerVLogGCFreq:         time.Minute * 1,
		BadgerMaxTableSize:       0,
		BadgerLevelOneSize:       0,
		BadgerLevSizeMultiplier:  0,
		BadgerKeepL0InMemory:     true,
		BadgerVLogFileSize:       0,
		BadgerVLogMaxEntries:     200000,
		BadgerUseLSMOnlyOptions:  true,
		BadgerEnableEventLogging: false,
		BadgerNumOfCompactors:    0,
		BadgerNumL0Tables:        0,
		BadgerNumL0TablesStall:   0,
		BadgerSyncWrites:         true,
		BadgerVLogFileIOMapping:  false,
		BadgerVLogTruncate:       true,
		EnableDeleteKeys:         false,
		EnableGranularMetrics:    false,
		BadgerDetailLogEnabled:   false,
	}
	return &defaultConfig
}

// This will first check if a config file is specified on cmd line using a temporary flagSet
// If not there, check the environment variable
// If we have a config path, load initial values from it
// Next parse flags again and override any fields from command line
//
// We do this to support settings that can come from either cmd line or config file
func Init() *SloopConfig {
	finalConfig := getDefaultConfig()
	configFileName := getConfigFilePath()
	if configFileName != "" {
		finalConfig = loadFromFile(configFileName, finalConfig)
	}
	registerFlags(flag.CommandLine, finalConfig)
	flag.Parse()
	// Set this to the correct value in case we got it from envVar
	finalConfig.ConfigFile = configFileName
	return finalConfig
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
	if c.DefaultLookback == "" {
		return fmt.Errorf("DefaultLookback can not be empty string")
	}
	_, err := time.ParseDuration(c.DefaultLookback)
	if err != nil {
		return errors.Wrapf(err, "DefaultLookback is an invalid duration: %v", c.DefaultLookback)
	}
	if c.CleanupFrequency < time.Minute*15 {
		return fmt.Errorf("CleanupFrequency can not be less than 15 minutes.  Badger is lazy about freeing space " +
			"on disk so we need to give it time to avoid over-correction")
	}
	return nil
}

func loadFromFile(filename string, config *SloopConfig) *SloopConfig {
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("failed to read %v. %v", filename, err))
	}

	if strings.Contains(filename, ".yaml") {
		err = yaml.Unmarshal(configFile, &config)
	} else if strings.Contains(filename, ".json") {
		err = json.Unmarshal(configFile, &config)
	} else {
		panic(fmt.Sprintf("incorrect file format %v. Use json or yaml file type. ", filename))
	}

	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal %v. %v", filename, err))
	}
	return config

}

func getConfigFilePath() string {
	configFileFlag := getConfigFlag()
	if configFileFlag != "" {
		glog.Infof("Config flag: %s", configFileFlag)
		return configFileFlag
	}

	configFileOS := os.Getenv(sloopConfigEnvVar)
	if configFileOS != "" {
		glog.Infof("Config env: %s", configFileOS)
		return configFileOS
	}

	glog.Infof("Default config set")
	return ""
}

// Pre-parse flags and return config filename without side-effects
func getConfigFlag() string {
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
