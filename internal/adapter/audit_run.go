package adapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
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

type auditCommand struct {
	Name        string
	Display     string
	Args        []string
	ParseCounts func(stdout string) map[string]int

	// ToolMissingHint, when non-empty, marks Name as an externally-installed companion tool
	// (e.g., govulncheck, cargo-audit). The helper does a PATH lookup first and returns
	// Skipped with this hint if the binary is missing. Empty for tools assumed to be present
	// because they're the project's own package manager (composer, npm, etc.).
	ToolMissingHint string
}

func executeAudit(ctx context.Context, workDir string, cmd auditCommand) AuditResult {
	if cmd.ToolMissingHint != "" {
		if _, err := exec.LookPath(cmd.Name); err != nil {
			return AuditResult{Skipped: true, SkipReason: cmd.ToolMissingHint}
		}
	}

	cmdLine := cmd.Display + " " + strings.Join(cmd.Args, " ")

	stdout, stderr, code, err := runAuditCommand(ctx, workDir, cmd.Name, cmd.Args)

	if err != nil {
		return AuditResult{
			CommandLine: cmdLine,
			Err:         err,
			Output:      mergeAuditOutput(stdout, stderr),
		}
	}

	counts := cmd.ParseCounts(stdout)

	return AuditResult{
		CommandLine:  cmdLine,
		ExitCode:     code,
		OK:           code == 0,
		SeverityRank: severityRankFromCounts(counts),
		Counts:       counts,
		Output:       mergeAuditOutput(stdout, stderr),
	}
}

var auditAllowlist = map[string]struct{}{
	"bun":          {},
	"bundle":       {},
	"bundle-audit": {},
	"cargo-audit":  {},
	"composer":     {},
	"go":           {},
	"govulncheck":  {},
	"npm":          {},
	"pnpm":         {},
	"pip-audit":    {},
	"uv":           {},
	"yarn":         {},
}

func runAuditCommand(ctx context.Context, workDir, name string, args []string) (stdout, stderr string, exitCode int, err error) {
	if _, ok := auditAllowlist[name]; !ok {
		return "", "", -1, fmt.Errorf("audit: command not allowed: %s", name)
	}

	path, lpErr := exec.LookPath(name)

	if lpErr != nil {
		return "", "", -1, lpErr
	}

	// #nosec G204 - binary name is allowlisted, args are built by PreFlight adapters
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = workDir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	exitCode = 0

	if runErr != nil {
		if ee, ok := errors.AsType[*exec.ExitError](runErr); ok {
			exitCode = ee.ExitCode()

			return stdout, stderr, exitCode, nil
		}

		return stdout, stderr, -1, runErr
	}

	return stdout, stderr, exitCode, nil
}

func severityRankFromCounts(counts map[string]int) int {
	if len(counts) == 0 {
		return 0
	}

	rank := 0

	for severity, count := range counts {
		if count <= 0 {
			continue
		}

		switch SeverityLevel(severity) {
		case 4:
			rank += 1000 * count
		case 3:
			rank += 100 * count
		case 2:
			rank += 10 * count
		case 1:
			rank += count
		default:
			rank += 5 * count
		}
	}

	return rank
}

func mergeAuditOutput(stdout, stderr string) string {
	stdoutTrimmed := strings.TrimSpace(stdout)
	stderrTrimmed := strings.TrimSpace(stderr)

	switch {
	case stdoutTrimmed != "" && stderrTrimmed != "":
		return stdoutTrimmed + "\n" + stderrTrimmed
	case stderrTrimmed != "":
		return stderrTrimmed
	default:
		return stdoutTrimmed
	}
}
