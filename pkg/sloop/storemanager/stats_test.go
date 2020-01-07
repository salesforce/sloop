package storemanager

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"path"
	"testing"
)

func Test_GetDirSizeRecursive(t *testing.T) {
	fs := afero.Afero{Fs: afero.NewMemMapFs()}
	fs.MkdirAll(someDir, 0700)
	// 3 vlog files
	fs.WriteFile(path.Join(someDir, "000010.vlog"), []byte("a"), 0700)
	fs.WriteFile(path.Join(someDir, "000011.vlog"), []byte("aa"), 0700)
	fs.WriteFile(path.Join(someDir, "000012.vlog"), []byte("aaaaa"), 0700)
	// 4 sst files
	fs.WriteFile(path.Join(someDir, "000070.sst"), []byte("zzzzzz"), 0700)
	fs.WriteFile(path.Join(someDir, "000071.sst"), []byte("zzzzzzz"), 0700)
	fs.WriteFile(path.Join(someDir, "000072.sst"), []byte("zzzzzzzz"), 0700)
	fs.WriteFile(path.Join(someDir, "000073.sst"), []byte("zzzzzzzzz"), 0700)
	// Other
	fs.WriteFile(path.Join(someDir, "KEYREGISTRY"), []byte("u"), 0700)
	fs.WriteFile(path.Join(someDir, "MANIFEST"), []byte("u"), 0700)

	subDir := path.Join(someDir, "subDir")
	fs.Mkdir(subDir, 0700)
	fs.WriteFile(path.Join(subDir, "randomFile"), []byte("abc"), 0700)

	fileSize, extFileCount, extByteCount, err := getDirSizeRecursive(someDir, &fs)
	assert.Nil(t, err)
	assert.Equal(t, uint64(43), fileSize)
	assert.Equal(t, map[string]int(map[string]int{"": 3, ".sst": 4, ".vlog": 3}), extFileCount)
	assert.Equal(t, map[string]uint64(map[string]uint64{"": 5, ".sst": 30, ".vlog": 8}), extByteCount)
}
