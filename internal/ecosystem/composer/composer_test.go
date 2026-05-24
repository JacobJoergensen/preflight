package composer

import (
	"slices"
	"testing"
)

func TestParseComposerJSON(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantDep []string
		wantDev []string
	}{
		{
			name:    "filters php and ext requirements, keeps and sorts the rest",
			raw:     `{"require":{"php":"^8.2","ext-mbstring":"*","vendor/pkg":"^1.0","another/lib":"^2.0"}}`,
			wantDep: []string{"another/lib", "vendor/pkg"},
		},
		{
			name:    "filters ext from dev requirements",
			raw:     `{"require-dev":{"ext-xdebug":"*","phpunit/phpunit":"^10"}}`,
			wantDev: []string{"phpunit/phpunit"},
		},
		{
			name: "empty manifest yields no dependencies",
			raw:  `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, devDeps, err := parseComposerJSON([]byte(tt.raw))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !slices.Equal(deps, tt.wantDep) {
				t.Errorf("dependencies = %v, want %v", deps, tt.wantDep)
			}

			if !slices.Equal(devDeps, tt.wantDev) {
				t.Errorf("devDependencies = %v, want %v", devDeps, tt.wantDev)
			}
		})
	}
}

func TestParseComposerAdvisoryFindings(t *testing.T) {
	raw := `{"advisories":{"vendor/a":[{"advisoryId":"PKSA-1","cve":"CVE-1","severity":"high","title":"Title 1","link":"https://example.test/1"}],"vendor/b":[{"advisoryId":"PKSA-2","severity":"critical"}]}}`

	findings := parseComposerAdvisoryFindings(raw)

	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(findings))
	}

	// Findings sort by descending severity, so the critical advisory comes first.
	if findings[0].ID != "PKSA-2" || findings[0].Severity != "critical" || findings[0].Package != "vendor/b" {
		t.Errorf("findings[0] = %+v", findings[0])
	}

	if findings[1].ID != "CVE-1" || findings[1].Severity != "high" || findings[1].Package != "vendor/a" {
		t.Errorf("findings[1] = %+v", findings[1])
	}

	if !slices.Equal(findings[1].Aliases, []string{"PKSA-1"}) {
		t.Errorf("findings[1].Aliases = %v, want [PKSA-1]", findings[1].Aliases)
	}

	if findings[1].URL != "https://example.test/1" || findings[1].Summary != "Title 1" {
		t.Errorf("findings[1] url/summary = %q / %q", findings[1].URL, findings[1].Summary)
	}
}

func TestParseComposerAdvisoryFindingsDefaultsSeverity(t *testing.T) {
	findings := parseComposerAdvisoryFindings(`{"advisories":{"vendor/a":[{"cve":"CVE-1"}]}}`)

	if len(findings) != 1 || findings[0].Severity != "moderate" {
		t.Fatalf("got %+v, want one moderate finding", findings)
	}
}

func TestParseComposerAdvisoryFindingsEmpty(t *testing.T) {
	if findings := parseComposerAdvisoryFindings(`{"advisories":{}}`); findings != nil {
		t.Errorf("got %v, want nil", findings)
	}
}

func TestParseComposerLicenses(t *testing.T) {
	raw := `{"name":"root","version":"1.0","license":["MIT"],"dependencies":{"vendor/b":{"version":"2.0","license":["GPL-3.0-only"]},"vendor/a":{"version":"1.0","license":["MIT"]}}}`

	packages := parseComposerLicenses(raw)

	if len(packages) != 2 {
		t.Fatalf("got %d packages, want 2", len(packages))
	}

	// Sorted by name, so vendor/a comes first.
	if packages[0].Name != "vendor/a" || packages[0].Version != "1.0" || packages[0].License != "MIT" {
		t.Errorf("packages[0] = %+v", packages[0])
	}

	if packages[1].Name != "vendor/b" || packages[1].License != "GPL-3.0-only" {
		t.Errorf("packages[1] = %+v", packages[1])
	}
}
