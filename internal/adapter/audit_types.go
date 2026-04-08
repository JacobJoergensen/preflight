package adapter

import (
	"context"
	"strings"
)

type AuditRunner interface {
	Adapter
	Audit(ctx context.Context, deps Dependencies) AuditResult
}

type AuditResult struct {
	Skipped      bool           // SkipReason is set when Skipped is true (e.g. missing manifest or optional tool).
	SkipReason   string         // CommandLine is a human-readable representation of what ran.
	CommandLine  string         // ExitCode is the process exit code (-1 if the process did not run).
	ExitCode     int            // OK is true when the audit reports no vulnerabilities (tool-specific).
	OK           bool           // SeverityRank orders items for display: higher = worse (e.g. critical > high).
	SeverityRank int            // Counts aggregates by severity name when parsing succeeded (optional).
	Counts       map[string]int // Output combines stdout/stderr for display (may be truncated by renderers).
	Output       string         // Err is set when the audit tool could not be started (missing binary, etc.).
	Err          error
}

// SeverityLevel maps a severity name to an ordered level (0 = unknown, 4 = critical).
func SeverityLevel(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "moderate", "medium":
		return 2
	case "low":
		return 1
	case "info":
		return 0
	default:
		return 0
	}
}
