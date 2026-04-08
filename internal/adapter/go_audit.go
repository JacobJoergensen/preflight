package adapter

import (
	"context"
	"encoding/json"
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
	counts := parseGovulncheckCounts(stdout)
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
