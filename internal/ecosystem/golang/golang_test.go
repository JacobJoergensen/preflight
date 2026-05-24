package golang

import (
	"slices"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
)

func TestParseGovulncheckFindings(t *testing.T) {
	stream := `{"osv":{"id":"GO-2021-0001","aliases":["CVE-2021-1111"],"summary":"Bad bug"}}
{"finding":{"osv":"GO-2021-0001","trace":[{"module":""}]}}
{"finding":{"osv":"GO-2021-0001","fixed_version":"v1.2.3","trace":[{"module":"example.com/m","version":"v1.0.0"}]}}`

	findings := parseGovulncheckFindings(stream)

	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}

	got := findings[0]

	if got.ID != "GO-2021-0001" || got.Severity != "high" {
		t.Errorf("id/severity = %q / %q", got.ID, got.Severity)
	}

	// The module-level record is preferred over the package-less one.
	if got.Package != "example.com/m" || got.Version != "v1.0.0" || got.FixedIn != "v1.2.3" {
		t.Errorf("package/version/fixedIn = %q / %q / %q", got.Package, got.Version, got.FixedIn)
	}

	if got.URL != "https://pkg.go.dev/vuln/GO-2021-0001" || got.Summary != "Bad bug" {
		t.Errorf("url/summary = %q / %q", got.URL, got.Summary)
	}

	if !slices.Equal(got.Aliases, []string{"CVE-2021-1111"}) {
		t.Errorf("aliases = %v, want [CVE-2021-1111]", got.Aliases)
	}
}

func TestParseGoLicenses(t *testing.T) {
	output := "google.golang.org/grpc,https://github.com/grpc/grpc-go/blob/master/LICENSE,Apache-2.0\n" +
		"github.com/x/y,https://github.com/x/y/blob/main/LICENSE,MIT\n"

	packages := parseGoLicenses(output)

	if len(packages) != 2 {
		t.Fatalf("got %d packages, want 2", len(packages))
	}

	// Sorted by import path.
	if packages[0].Name != "github.com/x/y" || packages[0].License != "MIT" {
		t.Errorf("packages[0] = %+v", packages[0])
	}

	if packages[1].Name != "google.golang.org/grpc" || packages[1].License != "Apache-2.0" {
		t.Errorf("packages[1] = %+v", packages[1])
	}
}

func TestParseGoMod(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantVersion string
		wantModules []string
	}{
		{
			name:        "block require collects every module and the go version",
			content:     "module example.com/app\n\ngo 1.22\n\nrequire (\n\tgithub.com/foo/bar v1.2.3\n\tgithub.com/baz/qux v0.1.0\n)\n",
			wantVersion: "1.22",
			wantModules: []string{"github.com/baz/qux", "github.com/foo/bar"},
		},
		{
			name:        "single-line require collects the module",
			content:     "module example.com/app\n\ngo 1.21\n\nrequire example.com/lib v1.0.0\n",
			wantVersion: "1.21",
			wantModules: []string{"example.com/lib"},
		},
		{
			name:        "missing go directive yields an empty version",
			content:     "module example.com/app\n\nrequire example.com/lib v1.0.0\n",
			wantVersion: "",
			wantModules: []string{"example.com/lib"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, modules := parseGoMod(tt.content)

			if version != tt.wantVersion {
				t.Errorf("version = %q, want %q", version, tt.wantVersion)
			}

			if !slices.Equal(modules, tt.wantModules) {
				t.Errorf("modules = %v, want %v", modules, tt.wantModules)
			}
		})
	}
}

func TestParseOutdated(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []ecosystem.OutdatedPackage
	}{
		{
			name:   "keeps a direct module with a newer update",
			output: `{"Path":"a","Version":"v1.0.0","Update":{"Version":"v1.1.0"}}`,
			want:   []ecosystem.OutdatedPackage{{Name: "a", Current: "v1.0.0", Latest: "v1.1.0"}},
		},
		{
			name:   "skips indirect modules",
			output: `{"Path":"b","Version":"v1.0.0","Indirect":true,"Update":{"Version":"v2.0.0"}}`,
			want:   nil,
		},
		{
			name:   "skips modules with no available update",
			output: `{"Path":"c","Version":"v1.0.0"}`,
			want:   nil,
		},
		{
			name:   "skips modules already at the latest version",
			output: `{"Path":"d","Version":"v1.0.0","Update":{"Version":"v1.0.0"}}`,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOutdated(ecosystem.RunContext{}, tt.output)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
