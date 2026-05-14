package adapter

import (
	"context"
	"encoding/json"
	"strings"
)

func (g GoModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	if !deps.Loader.LoadGoConfig().HasMod {
		return AuditResult{Skipped: true, SkipReason: "no go.mod"}
	}

	return executeAudit(ctx, deps.Loader.WorkDir, auditCommand{
		Name:            "govulncheck",
		Display:         "govulncheck",
		Args:            []string{"-json", "./..."},
		ParseCounts:     parseGovulncheckCounts,
		ToolMissingHint: "govulncheck not found on PATH (install: go install golang.org/x/vuln/cmd/govulncheck@latest)",
	})
}

func parseGovulncheckCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" {
		return nil
	}

	vulnCount := 0

	for line := range strings.SplitSeq(jsonText, "\n") {
		line = strings.TrimSpace(line)

		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var msg struct {
			Finding *json.RawMessage `json:"finding"`
		}

		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msg.Finding != nil {
			vulnCount++
		}
	}

	if vulnCount == 0 {
		return nil
	}

	return map[string]int{"high": vulnCount}
}
