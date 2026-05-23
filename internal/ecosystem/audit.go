package ecosystem

import (
	"context"
	goexec "os/exec"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
)

func (s *Spec) RunAudit(ctx context.Context, rc RunContext, detection Detection) AuditResult {
	probe := detection.Active.Audit

	if probe == nil {
		return AuditResult{Skipped: true, SkipReason: "audit not supported"}
	}

	tool := probe.Tool

	if tool == "" {
		tool = detection.Active.Command
	}

	if probe.ToolMissingHint != "" {
		if _, err := goexec.LookPath(tool); err != nil {
			return AuditResult{Skipped: true, SkipReason: probe.ToolMissingHint}
		}
	}

	commandLine := strings.TrimSpace(tool + " " + strings.Join(probe.Args, " "))

	result, err := exec.Capture(ctx, Gate, rc.WorkDir, tool, probe.Args...)
	if err != nil {
		return AuditResult{
			CommandLine: commandLine,
			Err:         err,
			Output:      mergeOutput(result.Stdout, result.Stderr),
		}
	}

	counts := probe.Parse(result.Stdout)

	return AuditResult{
		CommandLine:  commandLine,
		ExitCode:     result.ExitCode,
		OK:           result.ExitCode == 0,
		SeverityRank: severityRankFromCounts(counts),
		Counts:       counts,
		Output:       mergeOutput(result.Stdout, result.Stderr),
	}
}

func SeverityLevel(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "moderate", "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
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

func mergeOutput(stdout, stderr string) string {
	out := strings.TrimSpace(stdout)
	errOut := strings.TrimSpace(stderr)

	switch {
	case out != "" && errOut != "":
		return out + "\n" + errOut
	case errOut != "":
		return errOut
	default:
		return out
	}
}
