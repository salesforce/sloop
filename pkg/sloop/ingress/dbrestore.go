package ingress

import (
	"os"
	"runtime"

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

	err = db.Load(file, runtime.NumCPU())
	if err != nil {
		return errors.Wrapf(err, "failed to restore database from file: %q", filename)
	}

	return nil
}
