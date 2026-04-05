package fs

import (
	"io/fs"
	"os"
)

type OSFS struct{}

func (OSFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) } // #nosec G304
func (OSFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (OSFS) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }
func (OSFS) Stat(name string) (fs.FileInfo, error)        { return os.Stat(name) }
func (OSFS) ReadDir(name string) ([]fs.DirEntry, error)   { return os.ReadDir(name) }
func (OSFS) RemoveAll(path string) error                  { return os.RemoveAll(path) }
