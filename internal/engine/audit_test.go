package engine

import (
	"testing"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
)

func TestFilterAuditReportByIgnored(t *testing.T) {
	report := result.AuditReport{Items: []result.AuditItem{
		{
			ScopeID: "rust",
			OK:      false,
			Findings: []model.Finding{
				{ID: "RUSTSEC-1", Aliases: []string{"CVE-2023-9"}, Severity: "high"},
				{ID: "CVE-2023-1", Severity: "low"},
			},
		},
		{
			ScopeID:  "go",
			OK:       false,
			Findings: []model.Finding{{ID: "GO-1", Severity: "high"}},
		},
	}}

	// Suppress one finding by its alias (CVE-2023-9 → RUSTSEC-1) and the Go
	// finding by id, using mixed case to confirm matching is case-insensitive.
	filtered := filterAuditReportByIgnored(report, []string{"cve-2023-9", "GO-1"})

	rust := filtered.Items[0]
	if len(rust.Findings) != 1 || rust.Findings[0].ID != "CVE-2023-1" {
		t.Errorf("rust findings = %+v, want only CVE-2023-1", rust.Findings)
	}

	if rust.OK {
		t.Error("rust still has a finding, OK should remain false")
	}

	golang := filtered.Items[1]
	if len(golang.Findings) != 0 {
		t.Errorf("go findings = %+v, want none", golang.Findings)
	}

	if !golang.OK {
		t.Error("all go findings suppressed, OK should flip to true")
	}
}

func TestFilterAuditReportByIgnoredLeavesErrorsAndGaps(t *testing.T) {
	report := result.AuditReport{Items: []result.AuditItem{
		{ScopeID: "js", OK: false, ErrText: "tool failed"},
		{ScopeID: "python", OK: false}, // non-zero exit, but no parsed findings
	}}

	filtered := filterAuditReportByIgnored(report, []string{"CVE-2023-1"})

	for _, item := range filtered.Items {
		if item.OK {
			t.Errorf("%s OK flipped to true, but it had no findings to suppress", item.ScopeID)
		}
	}
}
