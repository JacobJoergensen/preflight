package adapter

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

func (p PackageModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	if !deps.Loader.LoadPackageConfig().HasConfig {
		return AuditResult{Skipped: true, SkipReason: "no package.json"}
	}

	packageManager, found := deps.Loader.DetectPackageManager(manifest.PackageTypeJS)

	if !found {
		return AuditResult{Skipped: true, SkipReason: "no JavaScript package manager detected"}
	}

	cmd := packageManager.Command()

	if cmd == "yarn" {
		if !isYarnBerry(deps.FS, deps.Loader.WorkDir) {
			return AuditResult{Skipped: true, SkipReason: "yarn 1 audit JSON is unsupported; upgrade to yarn 4 (Berry) for audit support"}
		}

		return executeAudit(ctx, deps.Loader.WorkDir, auditCommand{
			Name:        "yarn",
			Display:     "yarn npm audit",
			Args:        []string{"npm", "audit", "--json"},
			ParseCounts: parseYarnNpmAuditCounts,
		})
	}

	return executeAudit(ctx, deps.Loader.WorkDir, auditCommand{
		Name:        cmd,
		Display:     cmd,
		Args:        []string{"audit", "--json"},
		ParseCounts: parseNPMVulnerabilityCounts,
	})
}

func parseYarnNpmAuditCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "{") {
		return nil
	}

	var advisories map[string][]struct {
		Severity string `json:"severity"`
	}

	if err := json.Unmarshal([]byte(jsonText), &advisories); err != nil {
		return nil
	}

	counts := make(map[string]int)

	for _, list := range advisories {
		for _, advisory := range list {
			severity := strings.ToLower(strings.TrimSpace(advisory.Severity))

			if severity == "" {
				continue
			}

			counts[severity]++
		}
	}

	if len(counts) == 0 {
		return nil
	}

	return counts
}

func parseNPMVulnerabilityCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "{") {
		return nil
	}

	var root map[string]json.RawMessage

	if err := json.Unmarshal([]byte(jsonText), &root); err != nil {
		return nil
	}

	counts := make(map[string]int)

	if metadataRaw, ok := root["metadata"]; ok {
		var metadata struct {
			Vulnerabilities struct {
				Critical int `json:"critical"`
				High     int `json:"high"`
				Info     int `json:"info"`
				Low      int `json:"low"`
				Moderate int `json:"moderate"`
			} `json:"vulnerabilities"`
		}

		if err := json.Unmarshal(metadataRaw, &metadata); err == nil {
			vulnerabilities := metadata.Vulnerabilities

			addIfPositive := func(key string, count int) {
				if count > 0 {
					counts[key] = count
				}
			}

			addIfPositive("info", vulnerabilities.Info)
			addIfPositive("low", vulnerabilities.Low)
			addIfPositive("moderate", vulnerabilities.Moderate)
			addIfPositive("high", vulnerabilities.High)
			addIfPositive("critical", vulnerabilities.Critical)
		}
	}

	return counts
}
