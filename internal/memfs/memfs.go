// Package memfs provides a small in-memory fs.FS for tests, keyed by the exact
// (OS-native) paths callers pass. The standard library's testing/fstest.MapFS
// is read-only and rejects OS-native paths, so it cannot stand in for the
// writable fs.FS used across this codebase.
package memfs

import (
	"io/fs"
	"time"

	preflightfs "github.com/JacobJoergensen/preflight/internal/fs"
)

type FS struct {
	files map[string][]byte
}

// New returns an in-memory FS backed by files (path -> contents). A nil entry
// is an existing empty file, which is enough for presence checks.
func New(files map[string][]byte) FS {
	if files == nil {
		files = map[string][]byte{}
	}

	return FS{files: files}
}

func (m FS) ReadFile(name string) ([]byte, error) {
	if data, ok := m.files[name]; ok {
		return data, nil
	}

	return nil, fs.ErrNotExist
}

func (m FS) WriteFile(name string, data []byte, _ fs.FileMode) error {
	m.files[name] = data
	return nil
}

func (FS) MkdirAll(string, fs.FileMode) error { return nil }

func (m FS) Stat(name string) (fs.FileInfo, error) {
	if _, ok := m.files[name]; ok {
		return fileInfo{}, nil
	}

	return nil, fs.ErrNotExist
}

var _ preflightfs.FS = FS{}

type fileInfo struct{}

func (fileInfo) Name() string       { return "" }
func (fileInfo) Size() int64        { return 0 }
func (fileInfo) Mode() fs.FileMode  { return 0 }
func (fileInfo) ModTime() time.Time { return time.Time{} }
func (fileInfo) IsDir() bool        { return false }
func (fileInfo) Sys() any           { return nil }
