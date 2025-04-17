package ingress

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/klauspost/compress/zstd"
	"github.com/pkg/errors"
	"github.com/salesforce/sloop/pkg/sloop/store/typed"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

func TestDatabaseRestore(t *testing.T) {
	tests := []struct {
		name     string
		backupFn func(db badgerwrap.DB, path string) error
		wantErr  error
	}{
		{
			name:     "restore uncompressed database backup",
			backupFn: backupUncompressed,
		},
		{
			name:     "restore zstd-compressed database backup",
			backupFn: backupZstdCompressed,
		},
	}

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := createExampleDatabase()
			if err != nil {
				t.Fatalf("failed to create example database: %s", err)
			}
			defer db.Close()

			dbPath := filepath.Join(tmpDir, fmt.Sprintf("test-%d.db", i))
			err = tt.backupFn(db, dbPath)
			if err != nil {
				t.Fatalf("failed to backup example database: %s", err)
			}

			if err := DatabaseRestore(db, dbPath); err != tt.wantErr {
				t.Errorf("DatabaseRestore() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func backupUncompressed(db badgerwrap.DB, path string) error {
	w, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "failed to create file")
	}
	defer w.Close()

	_, err = db.Backup(w, 0)
	if err != nil {
		return errors.Wrapf(err, "failed to backup database")
	}

	return nil
}

func backupZstdCompressed(db badgerwrap.DB, path string) error {
	cf, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "failed to create file")
	}
	defer cf.Close()

	zw, err := zstd.NewWriter(cf)
	if err != nil {
		return errors.Wrapf(err, "failed to create zstd writer")
	}

	_, err = db.Backup(zw, 0)
	if err != nil {
		return errors.Wrapf(err, "failed to backup database")
	}
	zw.Close()

	return nil
}

func createExampleDatabase() (badgerwrap.DB, error) {
	rootPath, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create temporary directory for database:")
	}

	factory := &badgerwrap.BadgerFactory{}
	storeConfig := &untyped.Config{
		RootPath:                rootPath,
		ConfigPartitionDuration: time.Hour,
	}
	db, err := untyped.OpenStore(factory, storeConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init untyped store:")
	}

	wt := typed.OpenKubeWatchResultTable()
	err = db.Update(func(txn badgerwrap.Txn) error {
		txerr := wt.Set(txn,
			typed.NewWatchTableKey(
				"somePartition",
				"someKind",
				"someNamespace",
				"someName",
				time.UnixMicro(0)).String(),
			&typed.KubeWatchResult{Timestamp: &timestamp.Timestamp{
				Seconds: 0,
				Nanos:   0,
			}, Kind: "test", Payload: "test"})
		if txerr != nil {
			return txerr
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error updating database:")
	}

	return db, nil
}
