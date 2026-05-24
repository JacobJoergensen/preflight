package render

import (
	"testing"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
)

func TestProjectStatusLine(t *testing.T) {
	if got := projectStatusLine(1, 2, "reported errors"); got != "1 of 2 projects reported errors" {
		t.Errorf("got %q", got)
	}

	if got := projectStatusLine(1, 1, "failed"); got != "1 of 1 project failed" {
		t.Errorf("got %q", got)
	}
}

func TestStatusFromReport(t *testing.T) {
	tests := []struct {
		name   string
		report result.CheckReport
		want   string
	}{
		{
			name:   "errors require resolution",
			report: result.CheckReport{Items: []result.CheckItem{{Messages: []model.Message{{Severity: model.SeverityError}}}}},
			want:   "Check completed, please resolve.",
		},
		{
			name:   "warnings only",
			report: result.CheckReport{Items: []result.CheckItem{{Messages: []model.Message{{Severity: model.SeverityWarning}}}}},
			want:   "Check completed with warnings, please review.",
		},
		{
			name:   "all healthy",
			report: result.CheckReport{Items: []result.CheckItem{{Messages: []model.Message{{Severity: model.SeveritySuccess}}}}},
			want:   "Check completed successfully!",
		},
		{
			name:   "canceled",
			report: result.CheckReport{Canceled: true},
			want:   "Checks canceled.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, text := statusFromReport(tt.report)

			if text != tt.want {
				t.Errorf("text = %q, want %q", text, tt.want)
			}
		})
	}
}

func TestAuditStatusFromReport(t *testing.T) {
	tests := []struct {
		name   string
		report result.AuditReport
		want   string
	}{
		{"no audits ran", result.AuditReport{}, "No audits ran (no matching scopes or tools)"},
		{"tool error", result.AuditReport{Items: []result.AuditItem{{ErrText: "boom"}}}, "Audit completed with errors (tool missing or failed to run)"},
		{"vulnerabilities", result.AuditReport{Items: []result.AuditItem{{OK: false}}}, "Vulnerabilities or policy findings reported"},
		{"clean", result.AuditReport{Items: []result.AuditItem{{OK: true}}}, "No blocking audit issues"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, text := auditStatusFromReport(tt.report)

			if text != tt.want {
				t.Errorf("text = %q, want %q", text, tt.want)
			}
		})
	}
}

func TestFixStatusFromReport(t *testing.T) {
	tests := []struct {
		name   string
		report result.FixReport
		want   string
	}{
		{"aborted", result.FixReport{Aborted: true}, "Fix aborted — no changes applied"},
		{"dry run with plan", result.FixReport{DryRun: true, Plan: []result.PlannedFix{{ScopeID: "js"}}}, "Dry run completed, no changes made"},
		{"one failure", result.FixReport{Items: []result.FixItem{{Success: false}}}, "Fix completed with 1 failure"},
		{"all success", result.FixReport{Items: []result.FixItem{{Success: true}}}, "All dependencies fixed successfully"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, text := fixStatusFromReport(tt.report)

			if text != tt.want {
				t.Errorf("text = %q, want %q", text, tt.want)
			}
		})
	}
}
