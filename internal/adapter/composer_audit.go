package adapter

import (
	"context"
	"encoding/json"
	"strings"
)

func (c ComposerModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	composerConfig := deps.Loader.LoadComposerConfig()

	if !composerConfig.HasConfig {
		return AuditResult{Skipped: true, SkipReason: "no composer.json"}
	}

	workDir := deps.Loader.WorkDir
	args := []string{"audit", "--format=json"}
	cmdLine := "composer " + strings.Join(args, " ")

	stdout, stderr, code, err := runAuditCommand(ctx, workDir, "composer", args)

	if err != nil {
		return AuditResult{
			CommandLine: cmdLine,
			Err:         err,
			Output:      mergeAuditOutput(stdout, stderr),
		}
	}

	output := mergeAuditOutput(stdout, stderr)
	counts := parseComposerAdvisoryCounts(stdout)
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

func parseComposerAdvisoryCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "{") {
		return nil
	}

	var root struct {
		Advisories []json.RawMessage `json:"advisories"`
	}

	if err := json.Unmarshal([]byte(jsonText), &root); err != nil {
		return nil
	}

	counts := make(map[string]int)

	for _, raw := range root.Advisories {
		var advisory struct {
			Severity string `json:"severity"`
		}

		if err := json.Unmarshal(raw, &advisory); err != nil {
			continue
		}

		severity := strings.ToLower(strings.TrimSpace(advisory.Severity))

		if severity == "" {
			severity = "unknown"
		}

		counts[severity]++
	}

	return counts
}
