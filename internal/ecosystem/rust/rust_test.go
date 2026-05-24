package rust

import (
	"slices"
	"testing"
)

func TestAdvisorySeverity(t *testing.T) {
	tests := []struct {
		name          string
		informational string
		cvss          string
		want          string
	}{
		{
			name: "high-impact vector is critical",
			cvss: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			want: "critical",
		},
		{
			name: "changed scope pushes the score to critical",
			cvss: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
			want: "critical",
		},
		{
			name: "mid-range vector is moderate",
			cvss: "CVSS:3.1/AV:L/AC:L/PR:L/UI:N/S:U/C:H/I:N/A:N",
			want: "moderate",
		},
		{
			name:          "informational advisory is info even with a high cvss",
			informational: "unmaintained",
			cvss:          "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			want:          "info",
		},
		{
			name: "absent cvss is info",
			want: "info",
		},
		{
			name: "unparseable vector is info",
			cvss: "not-a-cvss-vector",
			want: "info",
		},
		{
			name: "zero-impact vector is info",
			cvss: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:N",
			want: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := advisorySeverity(tt.informational, tt.cvss)

			if got != tt.want {
				t.Errorf("advisorySeverity(%q, %q) = %q, want %q", tt.informational, tt.cvss, got, tt.want)
			}
		})
	}
}

func TestParseCargoToml(t *testing.T) {
	config := parseCargoToml([]byte(`[package]
rust-version = "1.75"

[dependencies]
serde = "1.0"
tokio = { version = "1", optional = true }

[dev-dependencies]
mockall = "0.12"
`))

	if config.RustVersion != "1.75" {
		t.Errorf("rust-version = %q, want 1.75", config.RustVersion)
	}

	if !slices.Equal(config.Dependencies, []string{"serde"}) {
		t.Errorf("dependencies = %v, want [serde]", config.Dependencies)
	}

	if !slices.Equal(config.OptionalDependencies, []string{"tokio"}) {
		t.Errorf("optionalDependencies = %v, want [tokio]", config.OptionalDependencies)
	}

	if !slices.Equal(config.DevDependencies, []string{"mockall"}) {
		t.Errorf("devDependencies = %v, want [mockall]", config.DevDependencies)
	}
}
