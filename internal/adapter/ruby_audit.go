package adapter

import (
	"context"
	"os/exec"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

func (r RubyModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	_, found := deps.Loader.DetectPackageManager(manifest.PackageTypeRuby)

	if !found {
		return AuditResult{Skipped: true, SkipReason: "no Gemfile or Ruby project detected"}
	}

	if _, err := exec.LookPath("bundle-audit"); err != nil {
		return AuditResult{
			Skipped:    true,
			SkipReason: "bundle-audit not found on PATH (gem install bundler-audit)",
		}
	}

	workDir := deps.Loader.WorkDir
	args := []string{"check"}
	cmdLine := "bundle-audit " + strings.Join(args, " ")

	stdout, stderr, code, err := runAuditCommand(ctx, workDir, "bundle-audit", args)

	if err != nil {
		return AuditResult{
			CommandLine: cmdLine,
			Err:         err,
			Output:      mergeAuditOutput(stdout, stderr),
		}
	}

	output := mergeAuditOutput(stdout, stderr)
	counts := parseBundleAuditCounts(output)
	rank := severityRankFromCounts(counts)
	passed := code == 0

	return AuditResult{
		CommandLine:  cmdLine,
		ExitCode:     code,
		OK:           passed,
		SeverityRank: rank,
		Counts:       counts,
		Output:       output,
	}
}

func parseBundleAuditCounts(output string) map[string]int {
	if output == "" {
		return nil
	}

	counts := make(map[string]int)

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)

		if !strings.HasPrefix(line, "Criticality:") {
			continue
		}

		severity := strings.TrimSpace(strings.TrimPrefix(line, "Criticality:"))
		severity = strings.ToLower(severity)

		switch severity {
		case "critical":
			counts["critical"]++
		case "high":
			counts["high"]++
		case "medium":
			counts["moderate"]++
		case "low":
			counts["low"]++
		default:
			counts["high"]++
		}
	}

	if len(counts) == 0 {
		return nil
	}

	return counts
}
