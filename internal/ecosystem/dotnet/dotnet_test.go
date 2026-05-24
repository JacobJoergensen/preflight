package dotnet

import (
	"testing"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
)

func TestParseDotnetVulnerabilities(t *testing.T) {
	raw := `{"version":1,"projects":[{"frameworks":[{"framework":"net8.0","topLevelPackages":[{"id":"Newtonsoft.Json","resolvedVersion":"12.0.1","vulnerabilities":[{"severity":"High","advisoryurl":"https://github.com/advisories/GHSA-5crp-9r3c-p9vr"}]}],"transitivePackages":[{"id":"System.Net.Http","resolvedVersion":"4.3.0","vulnerabilities":[{"severity":"Critical","advisoryurl":"https://github.com/advisories/GHSA-7jgj-8wvc-jh57"}]}]}]}]}`

	findings := parseDotnetVulnerabilities(raw)

	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(findings))
	}

	// Sorted by descending severity, so the critical transitive package is first.
	if findings[0].Package != "System.Net.Http" || findings[0].Severity != "critical" {
		t.Errorf("findings[0] = %+v", findings[0])
	}

	if findings[1].ID != "GHSA-5crp-9r3c-p9vr" || findings[1].Severity != "high" || findings[1].Package != "Newtonsoft.Json" {
		t.Errorf("findings[1] = %+v", findings[1])
	}

	if findings[1].Version != "12.0.1" || findings[1].URL != "https://github.com/advisories/GHSA-5crp-9r3c-p9vr" {
		t.Errorf("findings[1] version/url = %q / %q", findings[1].Version, findings[1].URL)
	}
}

func TestParseDotnetOutdated(t *testing.T) {
	raw := `{"version":1,"projects":[{"frameworks":[{"framework":"net8.0","topLevelPackages":[{"id":"Serilog","resolvedVersion":"2.0.0","latestVersion":"3.1.1"},{"id":"Polly","resolvedVersion":"8.0.0","latestVersion":"8.0.0"}]}]}]}`

	packages, err := parseOutdated(ecosystem.RunContext{}, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("got %d packages, want 1 (up-to-date Polly excluded)", len(packages))
	}

	if got := packages[0]; got.Name != "Serilog" || got.Current != "2.0.0" || got.Latest != "3.1.1" {
		t.Errorf("packages[0] = %+v", got)
	}
}
