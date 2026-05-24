package fs

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type MemFS struct {
	files map[string][]byte
}

func NewMemFS(files map[string][]byte) MemFS {
	if files == nil {
		files = map[string][]byte{}
	}

	return MemFS{files: files}
}

func (m MemFS) ReadFile(name string) ([]byte, error) {
	if data, ok := m.files[name]; ok {
		return data, nil
	}

	return nil, fs.ErrNotExist
}

func (m MemFS) WriteFile(name string, data []byte, _ fs.FileMode) error {
	m.files[name] = data
	return nil
}

func (MemFS) MkdirAll(string, fs.FileMode) error { return nil }

func (m MemFS) Stat(name string) (fs.FileInfo, error) {
	if _, ok := m.files[name]; ok {
		return memFileInfo{}, nil
	}

	return nil, fs.ErrNotExist
}

func (m MemFS) ReadDir(name string) ([]fs.DirEntry, error) {
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
		entries = append(entries, memDirEntry{name: child, dir: isDir})
	}

	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	return entries, nil
}

var _ FS = MemFS{}

type memFileInfo struct{}

func (memFileInfo) Name() string       { return "" }
func (memFileInfo) Size() int64        { return 0 }
func (memFileInfo) Mode() fs.FileMode  { return 0 }
func (memFileInfo) ModTime() time.Time { return time.Time{} }
func (memFileInfo) IsDir() bool        { return false }
func (memFileInfo) Sys() any           { return nil }

type memDirEntry struct {
	name string
	dir  bool
}

func (e memDirEntry) Name() string { return e.name }
func (e memDirEntry) IsDir() bool  { return e.dir }

func (e memDirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}

	return 0
}

func (memDirEntry) Info() (fs.FileInfo, error) { return memFileInfo{}, nil }
