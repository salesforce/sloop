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
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/salesforce/sloop/pkg/sloop/webserver"
)

const sloopConfigEnvVar = "SLOOP_CONFIG"

type SloopConfig struct {
	// These fields can only come from command line or flag args
	ConfigFile string	`json:"config"`
	// These fields can only come from file because they use complex types
	LeftBarLinks  []webserver.LinkTemplate         `json:"leftBarLinks"`
	ResourceLinks []webserver.ResourceLinkTemplate `json:"resourceLinks"`
	// Normal fields that can come from file or cmd line
	DisableKubeWatcher       bool          `json:"disable-kube-watch"`
	KubeWatchResyncInterval  time.Duration `json:"kube-watch-resync-interval"`
	WebFilesPath             string        `json:"webfiles-path"`
	BindAddress              string        `json:"bind-address"`
	Port                     int           `json:"port"`
	StoreRoot                string        `json:"store-root"`
	MaxLookback              time.Duration `json:"max-look-back"`
	MaxDiskMb                int           `json:"max-disk-mb"`
	DebugPlaybackFile        string        `json:"playback-file"`
	DebugRecordFile          string        `json:"record-file"`
	DeletionBatchSize        int           `json:"deletion-batch-size"`
	UseMockBadger            bool          `json:"use-mock-badger"`
	DisableStoreManager      bool          `json:"disable-store-manager"`
	CleanupFrequency         time.Duration `json:"cleanup-frequency" validate:"min=1h,max=120h"`
	KeepMinorNodeUpdates     bool          `json:"keep-minor-node-updates"`
	DefaultNamespace         string        `json:"default-namespace"`
	DefaultKind              string        `json:"default-kind"`
	DefaultLookback          string        `json:"default-lookback"`
	UseKubeContext           string        `json:"context"`
	DisplayContext           string        `json:"display-context"`
	ApiServerHost            string        `json:"apiserver-host"`
	WatchCrds                bool          `json:"watch-crds"`
	CrdRefreshInterval       time.Duration `json:"crd-refresh-interval"`
	ThresholdForGC           float64       `json:"gc-threshold"`
	RestoreDatabaseFile      string        `json:"restore-database-file"`
	BadgerDiscardRatio       float64       `json:"badger-discard-ratio"`
	BadgerVLogGCFreq         time.Duration `json:"badger-vlog-gc-freq"`
	BadgerMaxTableSize       int64         `json:"badger-max-table-size"`
	BadgerLevelOneSize       int64         `json:"badger-level-one-size"`
	BadgerLevSizeMultiplier  int           `json:"badger-level-size-multiplier"`
	BadgerKeepL0InMemory     bool          `json:"badger-keep-l0-in-memory"`
	BadgerVLogFileSize       int64         `json:"badger-vlog-file-size"`
	BadgerVLogMaxEntries     uint          `json:"badger-vlog-max-entries"`
	BadgerUseLSMOnlyOptions  bool          `json:"badger-use-lsm-only-options"`
	BadgerEnableEventLogging bool          `json:"badger-enable-event-logging"`
	BadgerNumOfCompactors    int           `json:"badger-number-of-compactors"`
	BadgerNumL0Tables        int           `json:"badger-number-of-level-zero-tables"`
	BadgerNumL0TablesStall   int           `json:"badger-number-of-zero-tables-stall"`
	BadgerSyncWrites         bool          `json:"badger-sync-writes"`
	BadgerVLogFileIOMapping  bool          `json:"badger-vlog-fileIO-mapping"`
	BadgerVLogTruncate       bool          `json:"badger-vlog-truncate"`
	EnableDeleteKeys         bool          `json:"enable-delete-keys"`
}

func registerDefaultFlags(fs *flag.FlagSet, config *SloopConfig) {
	fs.StringVar(&config.ConfigFile, "config", "", "Path to a yaml or json config file")
	fs.BoolVar(&config.DisableKubeWatcher, "disable-kube-watch", false, "Turn off kubernetes watch")
	fs.DurationVar(&config.KubeWatchResyncInterval, "kube-watch-resync-interval", 30*time.Minute,
		"OPTIONAL: Kubernetes watch resync interval")
	fs.StringVar(&config.WebFilesPath, "web-files-path", "./pkg/sloop/webserver/webfiles", "Path to web files")
	fs.StringVar(&config.BindAddress, "bind-address", "", "Web server bind ip address.")
	fs.IntVar(&config.Port, "port", 8080, "Web server port")
	fs.StringVar(&config.StoreRoot, "store-root", "./data", "Path to store history data")
	fs.DurationVar(&config.MaxLookback, "max-look-back", time.Duration(14*24)*time.Hour, "Max history data to keep")
	fs.IntVar(&config.MaxDiskMb, "max-disk-mb", 32*1024, "Max disk storage in MB")
	fs.StringVar(&config.DebugPlaybackFile, "playback-file", "", "Read watch data from a playback file")
	fs.StringVar(&config.DebugRecordFile, "record-file", "", "Record watch data to a playback file")
	fs.BoolVar(&config.UseMockBadger, "use-mock-badger", false, "Use a fake in-memory mock of badger")
	fs.BoolVar(&config.DisableStoreManager, "disable-store-manager", false, "Turn off store manager which is to clean up database")
	fs.DurationVar(&config.CleanupFrequency, "cleanup-frequency", time.Minute*30, "Frequency between subsequent runs for the database cleanup")
	fs.BoolVar(&config.KeepMinorNodeUpdates, "keep-minor-node-updates", false, "Keep all node updates even if change is only condition timestamps")
	fs.StringVar(&config.DefaultLookback, "default-lookback", "1h", "Default UX filter lookback")
	fs.StringVar(&config.DefaultKind, "default-kind", "_all", "Default UX filter kind")
	fs.StringVar(&config.DefaultNamespace, "default-namespace", "default", "Default UX filter namespace")
	fs.IntVar(&config.DeletionBatchSize, "deletion-batch-size", 1000, "Size of batch for deletion")
	fs.StringVar(&config.UseKubeContext, "context", "", "Use a specific kubernetes context")
	fs.StringVar(&config.DisplayContext, "display-context", "", "Use this to override the display context.  When running in k8s the context is empty string.  This lets you override that (mainly useful if you are running many copies of sloop on different clusters) ")
	fs.StringVar(&config.ApiServerHost, "apiserver-host", "", "Kubernetes API server endpoint")
	fs.BoolVar(&config.WatchCrds, "watch-crds", true, "Watch for activity for CRDs")
	fs.DurationVar(&config.CrdRefreshInterval, "crd-refresh-interval", time.Duration(5*time.Minute), "Frequency between CRD Informer refresh")
	fs.StringVar(&config.RestoreDatabaseFile, "restore-database-file", "", "Restore database from backup file into current context.")
	fs.Float64Var(&config.BadgerDiscardRatio, "badger-discard-ratio", 0.99, "Badger value log GC uses this value to decide if it wants to compact a vlog file. The lower the value of discardRatio the higher the number of !badger!move keys. And thus more the number of !badger!move keys, the size on disk keeps on increasing over time.")
	fs.Float64Var(&config.ThresholdForGC, "gc-threshold", 0.8, "Threshold for GC to start garbage collecting")
	fs.DurationVar(&config.BadgerVLogGCFreq, "badger-vlog-gc-freq", time.Minute*1, "Frequency of running badger's ValueLogGC")
	fs.Int64Var(&config.BadgerMaxTableSize, "badger-max-table-size", 0, "Max LSM table size in bytes.  0 = use badger default")
	fs.Int64Var(&config.BadgerLevelOneSize, "badger-level-one-size", 0, "The maximum total size for Level 1.  0 = use badger default")
	fs.IntVar(&config.BadgerLevSizeMultiplier, "badger-level-size-multiplier", 0, "The ratio between the maximum sizes of contiguous levels in the LSM.  0 = use badger default")
	fs.BoolVar(&config.BadgerKeepL0InMemory, "badger-keep-l0-in-memory", true, "Keeps all level 0 tables in memory for faster writes and compactions")
	fs.Int64Var(&config.BadgerVLogFileSize, "badger-vlog-file-size", 0, "Max size in bytes per value log file. 0 = use badger default")
	fs.UintVar(&config.BadgerVLogMaxEntries, "badger-vlog-max-entries", 200000, "Max number of entries per value log files. 0 = use badger default")
	fs.BoolVar(&config.BadgerUseLSMOnlyOptions, "badger-use-lsm-only-options", true, "Sets a higher valueThreshold so values would be collocated with LSM tree reducing vlog disk usage")
	fs.BoolVar(&config.BadgerEnableEventLogging, "badger-enable-event-logging", false, "Turns on badger event logging")
	fs.IntVar(&config.BadgerNumOfCompactors, "badger-number-of-compactors", 0, "Number of compactors for badger")
	fs.IntVar(&config.BadgerNumL0Tables, "badger-number-of-level-zero-tables", 0, "Number of level zero tables for badger")
	fs.IntVar(&config.BadgerNumL0TablesStall, "badger-number-of-zero-tables-stall", 0, "Number of Level 0 tables that once reached causes the DB to stall until compaction succeeds")
	fs.BoolVar(&config.BadgerSyncWrites, "badger-sync-writes", true, "Sync Writes ensures writes are synced to disk if set to true")
	fs.BoolVar(&config.EnableDeleteKeys, "enable-delete-keys", false, "Use delete prefixes instead of dropPrefix for GC")
	fs.BoolVar(&config.BadgerVLogFileIOMapping, "badger-vlog-fileIO-mapping", false, "Indicates which file loading mode should be used for the value log data, in memory constrained environments the value is recommended to be true")
	fs.BoolVar(&config.BadgerVLogTruncate, "badger-vlog-truncate", true, "Truncate value log if badger db offset is different from badger db size")
}

// This will first check if a config file is specified on cmd line using a temporary flagSet
// If not there, check the environment variable
// If we have a config path, load initial values from it
// Next parse flags again and override any fields from command line
//
// We do this to support settings that can come from either cmd line or config file
func Init() *SloopConfig {
	finalConfig := &SloopConfig{}
	registerDefaultFlags(flag.CommandLine, finalConfig)

	configFileName:=""
	configFileFlag := preParseConfigFlag()
	configFileOS := os.Getenv(sloopConfigEnvVar)

	if configFileFlag != "" {
		configFileName = configFileFlag
		glog.Infof("Config flag: %s", configFileFlag)
	} else if configFileOS!=""{
		configFileName=configFileOS
		glog.Infof("Config env: %s", configFileOS)
	}

	if configFileName != "" {
		finalConfig = loadFromFile(configFileName,finalConfig)
	}
	//register cmd line args
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

func loadFromFile(filename string,config *SloopConfig) *SloopConfig {
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

// Pre-parse flags and return config filename without side-effects
func preParseConfigFlag() string {
	tempCfg := &SloopConfig{}
	fs := flag.NewFlagSet("configFileOnly", flag.ContinueOnError)
	registerDefaultFlags(fs, tempCfg)
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
