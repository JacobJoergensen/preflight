package memfs

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	preflightfs "github.com/JacobJoergensen/preflight/internal/fs"
)

type FS struct {
	files map[string][]byte
}

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

func (m FS) ReadDir(name string) ([]fs.DirEntry, error) {
	prefix := name

	if prefix != "" && !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}

	seen := make(map[string]bool)

	var entries []fs.DirEntry

	for path := range m.files {
		if !strings.HasPrefix(path, prefix) {
			continue
		}

		rest := strings.TrimPrefix(path, prefix)

		if rest == "" {
			continue
		}

		child, _, isDir := strings.Cut(rest, string(filepath.Separator))

		if child == "" || seen[child] {
			continue
		}

		seen[child] = true
		entries = append(entries, dirEntry{name: child, dir: isDir})
	}

	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	return entries, nil
}

var _ preflightfs.FS = FS{}

type fileInfo struct{}

func (fileInfo) Name() string       { return "" }
func (fileInfo) Size() int64        { return 0 }
func (fileInfo) Mode() fs.FileMode  { return 0 }
func (fileInfo) ModTime() time.Time { return time.Time{} }
func (fileInfo) IsDir() bool        { return false }
func (fileInfo) Sys() any           { return nil }

type dirEntry struct {
	name string
	dir  bool
}

func (e dirEntry) Name() string { return e.name }
func (e dirEntry) IsDir() bool  { return e.dir }

func (e dirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}

	return 0
}

func (dirEntry) Info() (fs.FileInfo, error) { return fileInfo{}, nil }
