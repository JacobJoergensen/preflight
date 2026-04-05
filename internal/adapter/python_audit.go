package adapter

import (
	"context"
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
	rank := 0

	if code != 0 {
		rank = 500
	}

	passed := code == 0

	return AuditResult{
		CommandLine:  cmdLine,
		ExitCode:     code,
		OK:           passed,
		SeverityRank: rank,
		Output:       output,
	}
}
