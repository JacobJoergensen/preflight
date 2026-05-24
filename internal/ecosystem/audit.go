package ecosystem

import (
	"cmp"
	"context"
	goexec "os/exec"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/model"
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

	result, err := exec.Capture(ctx, rc.WorkDir, tool, probe.Args...)
	if err != nil {
		return AuditResult{
			CommandLine: commandLine,
			Err:         err,
			Output:      mergeOutput(result.Stdout, result.Stderr),
		}
	}

	findings := probe.Parse(result.Stdout)

	manifest := detection.Active.LockFile
	if manifest == "" {
		manifest = detection.Active.ConfigFile
	}

	return AuditResult{
		CommandLine:  commandLine,
		ExitCode:     result.ExitCode,
		OK:           result.ExitCode == 0,
		SeverityRank: SeverityRankFromFindings(findings),
		Findings:     findings,
		Manifest:     manifest,
		Output:       mergeOutput(result.Stdout, result.Stderr),
	}
}

func NormalizeSeverity(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "moderate", "medium":
		return "moderate"
	case "low":
		return "low"
	default:
		return "info"
	}
}

func CountsBySeverity(findings []model.Finding) map[string]int {
	if len(findings) == 0 {
		return nil
	}

	counts := make(map[string]int)

	for _, finding := range findings {
		counts[NormalizeSeverity(finding.Severity)]++
	}

	return counts
}

func SortFindings(findings []model.Finding) {
	slices.SortFunc(findings, func(a, b model.Finding) int {
		if diff := cmp.Compare(SeverityLevel(b.Severity), SeverityLevel(a.Severity)); diff != 0 {
			return diff
		}

		if diff := cmp.Compare(a.Package, b.Package); diff != 0 {
			return diff
		}

		return cmp.Compare(a.ID, b.ID)
	})
}

func SeverityRankFromFindings(findings []model.Finding) int {
	rank := 0

	for _, finding := range findings {
		switch SeverityLevel(finding.Severity) {
		case 4:
			rank += 1000
		case 3:
			rank += 100
		case 2:
			rank += 10
		case 1:
			rank++
		default:
			rank += 5
		}
	}

	return rank
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
