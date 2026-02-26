package ingress

import (
	"io"
	"os"
	"runtime"

	"github.com/golang/glog"
	"github.com/klauspost/compress/zstd"
	"github.com/pkg/errors"

	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

// DatabaseRestore restores the DB from a backup file created by webserver.backupHandler
func DatabaseRestore(db badgerwrap.DB, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return errors.Wrapf(err, "failed to load database restore file: %q", filename)
	}
	defer file.Close()

	zr, err := zstd.NewReader(file)
	if err != nil {
		return errors.Wrapf(err, "failed to initialize zstd reader")
	}
	defer zr.Close()

	err = db.Load(zr, runtime.NumCPU())
	if errors.Is(err, zstd.ErrMagicMismatch) {
		glog.V(2).Infof("database file is not compressed with zstd, will load without decompressing")

		// We already loaded the database, advancing the file offset. To load the database again,
		// we must reset the offset to the start.
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return errors.Wrapf(err, "failed to to rewind to start of database restore file: %q", filename)
		}
		err = db.Load(file, runtime.NumCPU())
	}
	if err != nil {
		return errors.Wrapf(err, "failed to restore database from file: %q", filename)
	}
	return nil
}
