package ecosystem

import (
	"io/fs"
	"testing"
	"time"

	"github.com/JacobJoergensen/preflight/internal/model"
)

func TestMissingLockfileWarning(t *testing.T) {
	composer := Manager{
		Command:     "composer",
		ConfigFile:  "composer.json",
		LockFile:    "composer.lock",
		InstallArgs: []string{"install"},
	}

	tests := []struct {
		name     string
		manager  Manager
		present  map[string]struct{}
		hasDeps  bool
		wantWarn bool
	}{
		{
			name:     "manifest present and lockfile missing with dependencies warns",
			manager:  composer,
			present:  map[string]struct{}{"composer.json": {}},
			hasDeps:  true,
			wantWarn: true,
		},
		{
			name:     "present lockfile is silent",
			manager:  composer,
			present:  map[string]struct{}{"composer.json": {}, "composer.lock": {}},
			hasDeps:  true,
			wantWarn: false,
		},
		{
			name:     "no dependencies is silent",
			manager:  composer,
			present:  map[string]struct{}{"composer.json": {}},
			hasDeps:  false,
			wantWarn: false,
		},
		{
			name:     "manager without a lockfile is silent",
			manager:  Manager{Command: "pip", ConfigFile: "requirements.txt"},
			present:  map[string]struct{}{"requirements.txt": {}},
			hasDeps:  true,
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := RunContext{FS: statFS{present: tt.present}}
			got := MissingLockfileWarning(rc, tt.manager, tt.hasDeps)

			if tt.wantWarn {
				if len(got) != 1 || got[0].Severity != model.SeverityWarning {
					t.Fatalf("expected one warning, got %v", got)
				}

				return
			}

			if len(got) != 0 {
				t.Fatalf("expected no messages, got %v", got)
			}
		})
	}
}

type statFS struct {
	present map[string]struct{}
}

func (s statFS) Stat(name string) (fs.FileInfo, error) {
	if _, ok := s.present[name]; ok {
		return statFileInfo{}, nil
	}

	return nil, fs.ErrNotExist
}

func (statFS) ReadFile(string) ([]byte, error)             { return nil, fs.ErrNotExist }
func (statFS) WriteFile(string, []byte, fs.FileMode) error { return nil }
func (statFS) MkdirAll(string, fs.FileMode) error          { return nil }
func (statFS) ReadDir(string) ([]fs.DirEntry, error)       { return nil, nil }

type statFileInfo struct{}

func (statFileInfo) Name() string       { return "" }
func (statFileInfo) Size() int64        { return 0 }
func (statFileInfo) Mode() fs.FileMode  { return 0 }
func (statFileInfo) ModTime() time.Time { return time.Time{} }
func (statFileInfo) IsDir() bool        { return false }
func (statFileInfo) Sys() any           { return nil }
