package manifest

import (
	"path/filepath"

	"github.com/JacobJoergensen/preflight/internal/fs"
)

type Loader struct {
	WorkDir string
	FS      fs.FS
}

func NewLoader(workDir string) Loader {
	if workDir == "" {
		workDir = "."
	}

	return Loader{WorkDir: workDir, FS: fs.OSFS{}}
}

func (l Loader) readFile(name string) ([]byte, error) {
	path := filepath.Join(l.WorkDir, name)
	return l.FS.ReadFile(path)
}

func (l Loader) FileExists(name string) bool {
	path := filepath.Join(l.WorkDir, name)
	_, err := l.FS.Stat(path)
	return err == nil
}
