package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
)

func TestSARIFAuditRendererEmpty(t *testing.T) {
	var buf bytes.Buffer

	if err := (SARIFAuditRenderer{Out: &buf}).Render(result.AuditReport{}); err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()

	if strings.Contains(out, `"results": null`) {
		t.Error("a clean audit must emit results: [], not null")
	}

	if !strings.Contains(out, `"results": []`) {
		t.Errorf("expected an empty results array, got:\n%s", out)
	}
}

func TestSARIFAuditRenderer(t *testing.T) {
	report := result.AuditReport{Items: []result.AuditItem{
		{
			ScopeID:  "rust",
			Manifest: "Cargo.lock",
			Findings: []model.Finding{
				{ID: "RUSTSEC-1", Severity: "critical", Package: "time", Version: "0.2.22", FixedIn: "0.2.23", URL: "https://rustsec.org/advisories/RUSTSEC-1", Summary: "Segfault"},
			},
		},
		{
			ScopeID:  "js",
			Project:  "packages/app",
			Manifest: "package-lock.json",
			Findings: []model.Finding{
				{ID: "GHSA-1", Severity: "moderate", Package: "lodash"},
			},
		},
	}}

	var buf bytes.Buffer

	if err := (SARIFAuditRenderer{Out: &buf, ToolVersion: "2.0.0"}).Render(report); err != nil {
		t.Fatalf("render: %v", err)
	}

	var doc sarifLog

	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid SARIF JSON: %v", err)
	}

	if doc.Version != "2.1.0" {
		t.Errorf("version = %q, want 2.1.0", doc.Version)
	}

	if len(doc.Runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(doc.Runs))
	}

	run := doc.Runs[0]

	if run.Tool.Driver.Name != "preflight" || run.Tool.Driver.Version != "2.0.0" {
		t.Errorf("driver = %+v", run.Tool.Driver)
	}

	if len(run.Tool.Driver.Rules) != 2 {
		t.Errorf("got %d rules, want 2", len(run.Tool.Driver.Rules))
	}

	if len(run.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(run.Results))
	}

	// Rust critical maps to error, located at the lockfile in the repo root.
	first := run.Results[0]
	if first.RuleID != "RUSTSEC-1" || first.Level != "error" {
		t.Errorf("first result: ruleId %q level %q", first.RuleID, first.Level)
	}

	if uri := first.Locations[0].PhysicalLocation.ArtifactLocation.URI; uri != "Cargo.lock" {
		t.Errorf("first location uri = %q, want Cargo.lock", uri)
	}

	// JS moderate maps to warning, located under the monorepo sub-project path.
	second := run.Results[1]
	if second.Level != "warning" {
		t.Errorf("second level = %q, want warning", second.Level)
	}

	if uri := second.Locations[0].PhysicalLocation.ArtifactLocation.URI; uri != "packages/app/package-lock.json" {
		t.Errorf("second location uri = %q, want packages/app/package-lock.json", uri)
	}

	for _, rule := range run.Tool.Driver.Rules {
		if rule.ID == "RUSTSEC-1" && rule.Properties.SecuritySeverity != "9.8" {
			t.Errorf("RUSTSEC-1 security-severity = %q, want 9.8", rule.Properties.SecuritySeverity)
		}
	}
}
