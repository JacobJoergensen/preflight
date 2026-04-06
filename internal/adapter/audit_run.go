package adapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var auditAllowlist = map[string]struct{}{
	"bun":          {},
	"bundle":       {},
	"bundle-audit": {},
	"composer":     {},
	"go":           {},
	"govulncheck":  {},
	"npm":          {},
	"pnpm":         {},
	"pip-audit":    {},
	"yarn":         {},
}

func runAuditCommand(ctx context.Context, workDir, name string, args []string) (stdout, stderr string, exitCode int, err error) {
	if _, ok := auditAllowlist[name]; !ok {
		return "", "", -1, fmt.Errorf("audit: command not allowed: %s", name)
	}

	path, lpErr := exec.LookPath(name)

	if lpErr != nil {
		return "", "", -1, lpErr
	}

	// #nosec G204 - binary name is allowlisted, args are built by PreFlight adapters
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = workDir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	exitCode = 0

	if runErr != nil {
		if ee, ok := errors.AsType[*exec.ExitError](runErr); ok {
			exitCode = ee.ExitCode()

			return stdout, stderr, exitCode, nil
		}

		return stdout, stderr, -1, runErr
	}

	return stdout, stderr, exitCode, nil
}

func severityRankFromCounts(counts map[string]int) int {
	if len(counts) == 0 {
		return 0
	}

	rank := 0

	for severity, count := range counts {
		if count <= 0 {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(severity)) {
		case "critical":
			rank += 1000 * count
		case "high":
			rank += 100 * count
		case "moderate", "medium":
			rank += 10 * count
		case "low":
			rank += count
		case "info":
			rank += 0
		default:
			rank += 5 * count
		}
	}

	return rank
}

func mergeAuditOutput(stdout, stderr string) string {
	stdoutTrimmed := strings.TrimSpace(stdout)
	stderrTrimmed := strings.TrimSpace(stderr)

	switch {
	case stdoutTrimmed != "" && stderrTrimmed != "":
		return stdoutTrimmed + "\n" + stderrTrimmed
	case stderrTrimmed != "":
		return stderrTrimmed
	default:
		return stdoutTrimmed
	}
}
