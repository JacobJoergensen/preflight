package adapter

import (
	"context"
	"encoding/json"
	"strings"
)

func (r RustModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	if !deps.Loader.LoadCargoConfig().HasManifest {
		return AuditResult{Skipped: true, SkipReason: "no Cargo.toml"}
	}

	return executeAudit(ctx, deps.Loader.WorkDir, auditCommand{
		Name:            "cargo-audit",
		Display:         "cargo audit",
		Args:            []string{"audit", "--json"},
		ParseCounts:     parseCargoAuditCounts,
		ToolMissingHint: "cargo-audit not found on PATH (install: cargo install cargo-audit)",
	})
}

func parseCargoAuditCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" {
		return nil
	}

	var report struct {
		Vulnerabilities struct {
			List []struct {
				Advisory struct {
					Severity string `json:"severity"`
				} `json:"advisory"`
			} `json:"list"`
		} `json:"vulnerabilities"`
	}

	if err := json.Unmarshal([]byte(jsonText), &report); err != nil {
		return nil
	}

	counts := make(map[string]int)

	for _, vuln := range report.Vulnerabilities.List {
		severity := strings.ToLower(strings.TrimSpace(vuln.Advisory.Severity))

		if severity == "" {
			severity = "info"
		}

		counts[severity]++
	}

	if len(counts) == 0 {
		return nil
	}

	return counts
}
