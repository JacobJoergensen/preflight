package adapter

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

func (p PythonModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	_, found := deps.Loader.DetectPackageManager(manifest.PackageTypePython)

	if !found {
		return AuditResult{Skipped: true, SkipReason: "no Python project detected"}
	}

	if _, err := exec.LookPath("pip-audit"); err != nil {
		return AuditResult{
			Skipped:    true,
			SkipReason: "pip-audit not found on PATH (install: pip install pip-audit)",
		}
	}

	workDir := deps.Loader.WorkDir
	args := []string{"--format", "json"}
	cmdLine := "pip-audit " + strings.Join(args, " ")

	stdout, stderr, code, err := runAuditCommand(ctx, workDir, "pip-audit", args)

	if err != nil {
		return AuditResult{
			CommandLine: cmdLine,
			Err:         err,
			Output:      mergeAuditOutput(stdout, stderr),
		}
	}

	output := mergeAuditOutput(stdout, stderr)
	counts := parsePipAuditCounts(stdout)
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
