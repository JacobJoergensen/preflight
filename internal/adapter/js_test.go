package adapter

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
)

type fakePackageFS struct {
	files map[string][]byte
}

func (f fakePackageFS) ReadFile(name string) ([]byte, error) {
	if data, ok := f.files[name]; ok {
		return data, nil
	}

	return nil, fs.ErrNotExist
}

func (f fakePackageFS) WriteFile(string, []byte, fs.FileMode) error { return nil }
func (f fakePackageFS) MkdirAll(string, fs.FileMode) error          { return nil }
func (f fakePackageFS) RemoveAll(string) error                      { return nil }
func (f fakePackageFS) Stat(string) (fs.FileInfo, error)            { return nil, fs.ErrNotExist }
func (f fakePackageFS) ReadDir(string) ([]fs.DirEntry, error)       { return nil, nil }

func TestGetInstalledPackagesOmitsUnreadableDeps(t *testing.T) {
	workDir := filepath.Join("project")

	fsys := fakePackageFS{files: map[string][]byte{
		filepath.Join(workDir, "node_modules", "react", "package.json"):                           []byte(`{"version":"18.3.0"}`),
		filepath.Join(workDir, "node_modules", "@rollup", "rollup-linux-x64-gnu", "package.json"): []byte(`{"version":"4.60.3"}`),
	}}

	installed := getInstalledPackages(
		context.Background(),
		fsys,
		workDir,
		[]string{"react", "missing-prod"},
		[]string{"missing-dev"},
		[]string{"@rollup/rollup-linux-x64-gnu", "@rollup/rollup-darwin-arm64"},
	)

	wantPresent := map[string]string{
		"react":                        "18.3.0",
		"@rollup/rollup-linux-x64-gnu": "4.60.3",
	}

	for name, wantVersion := range wantPresent {
		gotVersion, ok := installed[name]

		if !ok {
			t.Errorf("expected %q in installed map, missing", name)
			continue
		}

		if gotVersion != wantVersion {
			t.Errorf("installed[%q] = %q, want %q", name, gotVersion, wantVersion)
		}
	}

	wantAbsent := []string{"missing-prod", "missing-dev", "@rollup/rollup-darwin-arm64"}

	for _, name := range wantAbsent {
		if _, ok := installed[name]; ok {
			t.Errorf("expected %q absent from installed map (no package.json), but it was present", name)
		}
	}
}

func TestOptionalDepMatchesPlatform(t *testing.T) {
	tests := []struct {
		name    string
		depName string
		goos    string
		goarch  string
		want    bool
	}{
		{"linux-x64-gnu on linux amd64", "@rollup/rollup-linux-x64-gnu", "linux", "amd64", true},
		{"linux-x64-gnu on windows amd64", "@rollup/rollup-linux-x64-gnu", "windows", "amd64", false},
		{"linux-x64-gnu on darwin arm64", "@rollup/rollup-linux-x64-gnu", "darwin", "arm64", false},
		{"darwin-arm64 on darwin arm64", "@rollup/rollup-darwin-arm64", "darwin", "arm64", true},
		{"darwin-arm64 on darwin amd64", "@rollup/rollup-darwin-arm64", "darwin", "amd64", false},
		{"win32-x64 on windows amd64", "@esbuild/win32-x64", "windows", "amd64", true},
		{"win32-x64 on linux amd64", "@esbuild/win32-x64", "linux", "amd64", false},
		{"unscoped linux-x64-gnu on windows", "lightningcss-linux-x64-gnu", "windows", "amd64", false},
		{"no platform tokens (react)", "react", "windows", "amd64", true},
		{"no platform tokens (fsevents)", "fsevents", "linux", "amd64", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optionalDepMatchesPlatform(tt.depName, tt.goos, tt.goarch)

			if got != tt.want {
				t.Errorf("optionalDepMatchesPlatform(%q, %q, %q) = %v, want %v", tt.depName, tt.goos, tt.goarch, got, tt.want)
			}
		})
	}
}
