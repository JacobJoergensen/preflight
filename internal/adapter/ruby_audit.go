package adapter

import (
	"context"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

func (r RubyModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	if _, found := deps.Loader.DetectPackageManager(manifest.PackageTypeRuby); !found {
		return AuditResult{Skipped: true, SkipReason: "no Gemfile or Ruby project detected"}
	}

	return executeAudit(ctx, deps.Loader.WorkDir, auditCommand{
		Name:            "bundle-audit",
		Display:         "bundle-audit",
		Args:            []string{"check"},
		ParseCounts:     parseBundleAuditCounts,
		ToolMissingHint: "bundle-audit not found on PATH (gem install bundler-audit)",
	})
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
