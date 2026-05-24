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

func TestParseCargoAuditFindings(t *testing.T) {
	raw := `{"vulnerabilities":{"list":[{"advisory":{"id":"RUSTSEC-2021-0001","package":"time","title":"Segfault","url":"https://rustsec.org/advisories/RUSTSEC-2021-0001","aliases":["CVE-2020-26235"],"cvss":"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"},"versions":{"patched":[">=0.2.23"]},"package":{"name":"time","version":"0.2.22"}}]}}`

	findings := parseCargoAuditFindings(raw)

	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}

	got := findings[0]

	if got.ID != "RUSTSEC-2021-0001" || got.Severity != "critical" || got.Package != "time" {
		t.Errorf("id/severity/package = %q / %q / %q", got.ID, got.Severity, got.Package)
	}

	if got.Version != "0.2.22" || got.FixedIn != ">=0.2.23" {
		t.Errorf("version/fixedIn = %q / %q", got.Version, got.FixedIn)
	}

	if got.URL != "https://rustsec.org/advisories/RUSTSEC-2021-0001" || got.Summary != "Segfault" {
		t.Errorf("url/summary = %q / %q", got.URL, got.Summary)
	}

	if !slices.Equal(got.Aliases, []string{"CVE-2020-26235"}) {
		t.Errorf("aliases = %v, want [CVE-2020-26235]", got.Aliases)
	}
}

func TestParseCargoLicenses(t *testing.T) {
	raw := `{"packages":[{"id":"root 1.0","name":"root","version":"1.0","license":"MIT"},{"id":"serde 1.0","name":"serde","version":"1.0","license":"MIT OR Apache-2.0"}],"workspace_members":["root 1.0"]}`

	packages := parseCargoLicenses(raw)

	if len(packages) != 1 {
		t.Fatalf("got %d packages, want 1 (workspace member excluded)", len(packages))
	}

	if packages[0].Name != "serde" || packages[0].Version != "1.0" || packages[0].License != "MIT OR Apache-2.0" {
		t.Errorf("packages[0] = %+v", packages[0])
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
