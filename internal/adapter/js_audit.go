package adapter

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

func (p PackageModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	config := deps.Loader.LoadPackageConfig()

	if !config.HasConfig {
		return AuditResult{Skipped: true, SkipReason: "no package.json"}
	}

	packageManager, found := deps.Loader.DetectPackageManager(manifest.PackageTypeJS)

	if !found {
		return AuditResult{Skipped: true, SkipReason: "no JavaScript package manager detected"}
	}

	command := packageManager.Command()
	workDir := deps.Loader.WorkDir
	args := []string{"audit", "--json"}
	cmdLine := command + " " + strings.Join(args, " ")

	stdout, stderr, code, err := runAuditCommand(ctx, workDir, command, args)

	if err != nil {
		return AuditResult{
			CommandLine: cmdLine,
			Err:         err,
			Output:      mergeAuditOutput(stdout, stderr),
		}
	}

	output := mergeAuditOutput(stdout, stderr)
	counts := parseNPMVulnerabilityCounts(stdout)
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
