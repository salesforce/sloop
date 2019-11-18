package ingress

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
)

func TestDatabaseRestore(t *testing.T) {
	tempFile, err := ioutil.TempFile("/tmp", "sloop-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())

	type args struct {
		db       badgerwrap.DB
		filename string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "happy",
			args: args{
				db:       &badgerwrap.MockDb{},
				filename: tempFile.Name(),
			},
		},
		{
			name: "unknown file",
			args: args{
				db:       &badgerwrap.MockDb{},
				filename: "/tmp/unknown-file-doesnt-exist",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DatabaseRestore(tt.args.db, tt.args.filename); (err != nil) != tt.wantErr {
				t.Errorf("DatabaseRestore() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
