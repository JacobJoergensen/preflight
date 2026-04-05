package run

import (
	"io/fs"
	"slices"
	"testing"
	"time"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/manifest"
)

type fakeFileInfo struct{}

type fakeFS struct {
	files map[string][]byte
}

func (fakeFileInfo) Name() string {
	return ""
}

func (fakeFileInfo) Size() int64 {
	return 0
}

func (fakeFileInfo) Mode() fs.FileMode {
	return 0
}

func (fakeFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (fakeFileInfo) IsDir() bool {
	return false
}

func (fakeFileInfo) Sys() any {
	return nil
}

func (f fakeFS) ReadFile(name string) ([]byte, error) {
	if data, ok := f.files[name]; ok {
		return data, nil
	}

	return nil, fs.ErrNotExist
}

func (f fakeFS) WriteFile(string, []byte, fs.FileMode) error {
	return nil
}

func (f fakeFS) MkdirAll(string, fs.FileMode) error {
	return nil
}

func (f fakeFS) RemoveAll(string) error {
	return nil
}

func (f fakeFS) Stat(name string) (fs.FileInfo, error) {
	if _, ok := f.files[name]; ok {
		return fakeFileInfo{}, nil
	}

	return nil, fs.ErrNotExist
}

func (f fakeFS) ReadDir(string) ([]fs.DirEntry, error) {
	return nil, nil
}

func TestResolveScript(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string][]byte
		target    config.ScriptTarget
		wantBin   string
		wantArgs  []string
		wantError bool
	}{
		{
			name:      "validation fails when no target set",
			target:    config.ScriptTarget{},
			wantError: true,
		},
		{
			name:     "js returns npm run when package.json exists",
			files:    map[string][]byte{"package.json": {}},
			target:   config.ScriptTarget{JS: "test"},
			wantBin:  "npm",
			wantArgs: []string{"run", "test"},
		},
		{
			name:     "js returns yarn run when yarn.lock exists",
			files:    map[string][]byte{"package.json": {}, "yarn.lock": {}},
			target:   config.ScriptTarget{JS: "build"},
			wantBin:  "yarn",
			wantArgs: []string{"run", "build"},
		},
		{
			name:     "composer returns composer run-script",
			target:   config.ScriptTarget{Composer: "test"},
			wantBin:  "composer",
			wantArgs: []string{"run-script", "test"},
		},
		{
			name:     "go without prefix returns go with args",
			target:   config.ScriptTarget{Go: "test ./..."},
			wantBin:  "go",
			wantArgs: []string{"test", "./..."},
		},
		{
			name:     "go with prefix strips go from args",
			target:   config.ScriptTarget{Go: "go build -o bin/app"},
			wantBin:  "go",
			wantArgs: []string{"build", "-o", "bin/app"},
		},
		{
			name:      "go with empty value fails",
			target:    config.ScriptTarget{Go: "   "},
			wantError: true,
		},
		{
			name:     "ruby returns bundle exec",
			target:   config.ScriptTarget{Ruby: "rake test"},
			wantBin:  "bundle",
			wantArgs: []string{"exec", "rake", "test"},
		},
		{
			name:      "ruby with empty value fails",
			target:    config.ScriptTarget{Ruby: "   "},
			wantError: true,
		},
		{
			name:     "python with poetry returns poetry run",
			files:    map[string][]byte{"pyproject.toml": []byte("[tool.poetry]"), "poetry.lock": {}},
			target:   config.ScriptTarget{Python: "pytest"},
			wantBin:  "poetry",
			wantArgs: []string{"run", "pytest"},
		},
		{
			name:     "python with pip returns python directly",
			files:    map[string][]byte{"requirements.txt": {}},
			target:   config.ScriptTarget{Python: "pytest -v"},
			wantBin:  "python",
			wantArgs: []string{"pytest", "-v"},
		},
		{
			name:      "python fails when no package manager detected",
			target:    config.ScriptTarget{Python: "pytest"},
			wantError: true,
		},
		{
			name:      "python with empty value fails",
			files:     map[string][]byte{"requirements.txt": {}},
			target:    config.ScriptTarget{Python: "   "},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := manifest.Loader{
				WorkDir: "",
				FS:      fakeFS{files: tt.files},
			}

			bin, args, err := ResolveScript(loader, tt.target)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if bin != tt.wantBin {
				t.Errorf("bin = %q, want %q", bin, tt.wantBin)
			}

			if !slices.Equal(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}
