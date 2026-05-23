package golang

import (
	"slices"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
)

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
