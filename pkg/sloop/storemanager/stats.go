package storemanager

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
	"github.com/spf13/afero"
	"os"
	"path/filepath"
	"time"
)

const vlogExt = ".vlog" // value log data
const sstExt = ".sst"   // LSM data

var (
	metricStoreSizeOnDiskMb   = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_store_sizeondiskmb"})
	metricBadgerKeys          = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "sloop_badger_keys"}, []string{"level"})
	metricBadgerTables        = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "sloop_badger_tables"}, []string{"level"})
	metricBadgerLsmFileCount  = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_badger_lsmfilecount"})
	metricBadgerLsmSizeMb     = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_badger_lsmsizemb"})
	metricBadgerVlogFileCount = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_badger_vlogfilecount"})
	metricBadgerVlogSizeMb    = promauto.NewGauge(prometheus.GaugeOpts{Name: "sloop_badger_vlogsizemb"})
)

type storeStats struct {
	Timestamp         time.Time
	DiskSizeBytes     uint64
	DiskLsmBytes      uint64
	DiskLsmFileCount  int
	DiskVlogBytes     uint64
	DiskVlogFileCount int
	LevelToKeyCount   map[int]uint64
	LevelToTableCount map[int]int
}

func generateStats(storeRoot string, db badgerwrap.DB, fs *afero.Afero) *storeStats {
	ret := &storeStats{}
	ret.LevelToKeyCount = make(map[int]uint64)
	ret.LevelToTableCount = make(map[int]int)
	ret.Timestamp = time.Now()

	totalSizeBytes, extFileCount, extByteCount, err := getDirSizeRecursive(storeRoot, fs)
	if err != nil {
		// Swallowing on purpose as we still want the other stats
		glog.Errorf("Failed to check storage size on disk: %v", err)
	}
	ret.DiskSizeBytes = totalSizeBytes
	ret.DiskLsmFileCount = extFileCount[sstExt]
	ret.DiskLsmBytes = extByteCount[sstExt]
	ret.DiskVlogFileCount = extFileCount[vlogExt]
	ret.DiskVlogBytes = extByteCount[vlogExt]

	tables := db.Tables(true)
	for _, table := range tables {
		glog.V(2).Infof("BadgerDB TABLE id=%v keycount=%v level=%v left=%q right=%q", table.ID, table.KeyCount, table.Level, string(table.Left), string(table.Right))
		ret.LevelToTableCount[table.Level] += 1
		ret.LevelToKeyCount[table.Level] += table.KeyCount
	}

	glog.Infof("Finished updating store stats: %+v", ret)
	return ret
}

// Returns total size, count of files by extension, count of bytes by extension
func getDirSizeRecursive(root string, fs *afero.Afero) (uint64, map[string]int, map[string]uint64, error) {
	var totalSize uint64
	var extFileCount = make(map[string]int)
	var extByteCount = make(map[string]uint64)

	err := fs.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			totalSize += uint64(info.Size())
			ext := filepath.Ext(path)
			extFileCount[ext] += 1
			extByteCount[ext] += uint64(info.Size())
		}
		return nil
	})
	if err != nil {
		return 0, extFileCount, extByteCount, err
	}

	return totalSize, extFileCount, extByteCount, nil
}

func emitMetrics(stats *storeStats) {
	metricStoreSizeOnDiskMb.Set(float64(stats.DiskSizeBytes))
	for k, v := range stats.LevelToKeyCount {
		metricBadgerKeys.WithLabelValues(fmt.Sprintf("%v", k)).Set(float64(v))
	}
	for k, v := range stats.LevelToTableCount {
		metricBadgerTables.WithLabelValues(fmt.Sprintf("%v", k)).Set(float64(v))
	}
	metricBadgerLsmFileCount.Set(float64(stats.DiskLsmFileCount))
	metricBadgerLsmSizeMb.Set(float64(stats.DiskLsmBytes))
	metricBadgerVlogFileCount.Set(float64(stats.DiskVlogFileCount))
	metricBadgerVlogSizeMb.Set(float64(stats.DiskVlogBytes))
}
