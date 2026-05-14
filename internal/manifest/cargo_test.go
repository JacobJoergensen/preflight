package manifest

import (
	"slices"
	"testing"
)

func TestParseCargoToml(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantRustVersion  string
		wantDependencies []string
		wantDev          []string
		wantOptional     []string
	}{
		{
			name: "simple package with deps and dev-deps",
			input: `[package]
name = "myproject"
version = "0.1.0"
rust-version = "1.75"

[dependencies]
serde = "1.0"
tokio = "1"

[dev-dependencies]
mockito = "1.0"
`,
			wantRustVersion:  "1.75",
			wantDependencies: []string{"serde", "tokio"},
			wantDev:          []string{"mockito"},
		},
		{
			name: "inline-table dep with optional flag is routed to optional",
			input: `[package]
name = "myproject"

[dependencies]
serde = "1.0"
tokio = { version = "1", optional = true }
async-std = { version = "1", optional=true }
`,
			wantDependencies: []string{"serde"},
			wantOptional:     []string{"async-std", "tokio"},
		},
		{
			name: "comments and blank lines are ignored",
			input: `# top-level comment
[package]
name = "myproject" # inline comment
rust-version = "1.80" # another

[dependencies]
# a commented-out dep
# legacy = "0.1"
clap = "4.0"
`,
			wantRustVersion:  "1.80",
			wantDependencies: []string{"clap"},
		},
		{
			name: "unrelated sections ignored (workspace, bin, features)",
			input: `[workspace]
members = ["crates/*"]

[[bin]]
name = "mybin"

[features]
default = ["serde"]

[dependencies]
serde = "1.0"
`,
			wantDependencies: []string{"serde"},
		},
		{
			name: "missing [package] section yields empty rust-version",
			input: `[dependencies]
foo = "1.0"
`,
			wantDependencies: []string{"foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CargoConfig{}
			parseCargoToml(&config, tt.input)

			if config.RustVersion != tt.wantRustVersion {
				t.Errorf("RustVersion = %q, want %q", config.RustVersion, tt.wantRustVersion)
			}

			if !slices.Equal(config.Dependencies, tt.wantDependencies) {
				t.Errorf("Dependencies = %v, want %v", config.Dependencies, tt.wantDependencies)
			}

			if !slices.Equal(config.DevDependencies, tt.wantDev) {
				t.Errorf("DevDependencies = %v, want %v", config.DevDependencies, tt.wantDev)
			}

			if !slices.Equal(config.OptionalDependencies, tt.wantOptional) {
				t.Errorf("OptionalDependencies = %v, want %v", config.OptionalDependencies, tt.wantOptional)
			}
		})
	}
}
