package adapter

import (
	"context"
	"os/exec"
	"strings"
)

func (g GoModule) Audit(ctx context.Context, deps Dependencies) AuditResult {
	goConfig := deps.Loader.LoadGoConfig()

	if !goConfig.HasMod {
		return AuditResult{Skipped: true, SkipReason: "no go.mod"}
	}

	if _, err := exec.LookPath("govulncheck"); err != nil {
		return AuditResult{
			Skipped:    true,
			SkipReason: "govulncheck not found on PATH (install: go install golang.org/x/vuln/cmd/govulncheck@latest)",
		}
	}

	workDir := deps.Loader.WorkDir
	args := []string{"-json", "./..."}
	cmdLine := "govulncheck " + strings.Join(args, " ")

	stdout, stderr, code, err := runAuditCommand(ctx, workDir, "govulncheck", args)

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
