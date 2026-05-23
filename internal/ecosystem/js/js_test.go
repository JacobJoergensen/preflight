package js

import (
	"path/filepath"
	"testing"
)

func TestBuildPackagePath(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		wantOK   bool
		wantPath string
	}{
		{name: "plain package resolves under node_modules", pkg: "react", wantOK: true, wantPath: filepath.Join("node_modules", "react", "package.json")},
		{name: "scoped package resolves under node_modules", pkg: "@scope/pkg", wantOK: true, wantPath: filepath.Join("node_modules", "@scope", "pkg", "package.json")},
		{name: "parent traversal is rejected", pkg: "../evil", wantOK: false},
		{name: "unscoped slash is rejected", pkg: "foo/bar", wantOK: false},
		{name: "traversal inside a scope is rejected", pkg: "@scope/../evil", wantOK: false},
		{name: "nested path inside a scope is rejected", pkg: "@scope/sub/dir", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, ok := buildPackagePath(tt.pkg)

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if ok && path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestOptionalDepMatchesPlatform(t *testing.T) {
	tests := []struct {
		name   string
		pkg    string
		goos   string
		goarch string
		want   bool
	}{
		{name: "matching os and arch", pkg: "@esbuild/linux-x64", goos: "linux", goarch: "amd64", want: true},
		{name: "mismatched os", pkg: "@esbuild/darwin-arm64", goos: "linux", goarch: "amd64", want: false},
		{name: "mismatched arch", pkg: "@esbuild/linux-x64", goos: "linux", goarch: "arm64", want: false},
		{name: "no platform tokens matches anything", pkg: "react", goos: "linux", goarch: "amd64", want: true},
		{name: "win32 token maps to windows", pkg: "@foo/win32-x64", goos: "windows", goarch: "amd64", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := optionalDepMatchesPlatform(tt.pkg, tt.goos, tt.goarch); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkspacesConfigured(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "non-empty array is configured", raw: `{"workspaces":["packages/*"]}`, want: true},
		{name: "empty array is not configured", raw: `{"workspaces":[]}`, want: false},
		{name: "object form with packages is configured", raw: `{"workspaces":{"packages":["a"]}}`, want: true},
		{name: "absent workspaces is not configured", raw: `{"name":"app"}`, want: false},
		{name: "null workspaces is not configured", raw: `{"workspaces":null}`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workspacesConfigured([]byte(tt.raw)); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
