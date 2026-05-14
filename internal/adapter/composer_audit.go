package adapter

import (
	"context"
	"encoding/json"
	"strings"
)

func (c ComposerModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	if !deps.Loader.LoadComposerConfig().HasConfig {
		return AuditResult{Skipped: true, SkipReason: "no composer.json"}
	}

	return executeAudit(ctx, deps.Loader.WorkDir, auditCommand{
		Name:        "composer",
		Display:     "composer",
		Args:        []string{"audit", "--format=json"},
		ParseCounts: parseComposerAdvisoryCounts,
	})
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
