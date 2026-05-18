package adapter

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

func (p PythonModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	packageManager, found := deps.Loader.DetectPackageManager(manifest.PackageTypePython)

	if !found {
		return AuditResult{Skipped: true, SkipReason: "no Python project detected"}
	}

	if packageManager.Command() == "uv" {
		return executeAudit(ctx, deps.Loader.WorkDir, auditCommand{
			Name:        "uv",
			Display:     "uv audit",
			Args:        []string{"audit", "--format", "json"},
			ParseCounts: parseUvAuditCounts,
		})
	}

	return executeAudit(ctx, deps.Loader.WorkDir, auditCommand{
		Name:            "pip-audit",
		Display:         "pip-audit",
		Args:            []string{"--format", "json"},
		ParseCounts:     parsePipAuditCounts,
		ToolMissingHint: "pip-audit not found on PATH (install: pip install pip-audit)",
	})
}

func parsePipAuditCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "[") {
		return nil
	}

	var packages []struct {
		Vulns []struct {
			ID string `json:"id"`
		} `json:"vulns"`
	}

	if err := json.Unmarshal([]byte(jsonText), &packages); err != nil {
		return nil
	}

	vulnCount := 0

	for _, pkg := range packages {
		vulnCount += len(pkg.Vulns)
	}

	if vulnCount == 0 {
		return nil
	}

	return map[string]int{"high": vulnCount}
}

func parseUvAuditCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" {
		return nil
	}

	var report struct {
		Summary struct {
			Vulnerabilities int `json:"vulnerabilities"`
		} `json:"summary"`
	}

	if err := json.Unmarshal([]byte(jsonText), &report); err != nil {
		return nil
	}

	if report.Summary.Vulnerabilities == 0 {
		return nil
	}

	return map[string]int{"high": report.Summary.Vulnerabilities}
}
