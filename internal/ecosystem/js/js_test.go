package js

import (
	"path/filepath"
	"testing"
)

func TestParseNPMVulnerabilityFindings(t *testing.T) {
	raw := `{"vulnerabilities":{"lodash":{"name":"lodash","severity":"high","via":[{"source":1065,"name":"lodash","title":"Prototype Pollution","url":"https://github.com/advisories/GHSA-jf85-cpcp-j695","severity":"high","range":"<4.17.19"}]},"minimist":{"name":"minimist","severity":"moderate","via":["lodash"]}}}`

	findings := parseNPMVulnerabilityFindings(raw)

	// The string `via` entry (minimist → lodash) is a transitive link, not an
	// advisory, so only the object entry yields a finding.
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}

	got := findings[0]

	if got.ID != "GHSA-jf85-cpcp-j695" {
		t.Errorf("id = %q, want GHSA-jf85-cpcp-j695", got.ID)
	}

	if got.Severity != "high" || got.Package != "lodash" {
		t.Errorf("severity/package = %q / %q", got.Severity, got.Package)
	}

	if got.URL != "https://github.com/advisories/GHSA-jf85-cpcp-j695" || got.Summary != "Prototype Pollution" {
		t.Errorf("url/summary = %q / %q", got.URL, got.Summary)
	}
}

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
