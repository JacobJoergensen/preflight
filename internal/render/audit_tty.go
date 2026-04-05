package render

import (
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const auditOutputMaxRunes = 8000

type TTYAuditRenderer struct {
	Out *terminal.OutputWriter
}

func (r TTYAuditRenderer) Render(report result.AuditReport) error {
	ow := r.Out

	if ow == nil {
		ow = terminal.NewOutputWriter()
	}

	if terminal.Quiet {
		return nil
	}

	ow.Println(terminal.Bold + terminal.Blue + "\n╭─────────────────────────────────────────╮" + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "│" + terminal.Cyan + terminal.Bold + "  Security audit (native tools)  " + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "╰─────────────────────────────────────────╯" + terminal.Reset)
	ow.Println("")

	for _, item := range report.Items {
		renderAuditCardTTY(ow, item)
	}

	statusIcon, statusColor, statusText := auditStatusFromReport(report)
	ow.Println(terminal.Bold + "\n" + statusColor + statusIcon + " " + statusText + terminal.Reset)

	return nil
}

func renderAuditCardTTY(ow *terminal.OutputWriter, item result.AuditItem) {
	title := item.ScopeDisplay

	if title == "" {
		title = item.ScopeID
	}

	color := terminal.Green
	status := "PASS"

	switch {
	case item.Skipped:
		color = terminal.Yellow
		status = "SKIP"
	case item.ErrText != "":
		color = terminal.Red
		status = "ERROR"
	case !item.OK:
		color = terminal.Red
		status = "ISSUES"
	}

	header := fmt.Sprintf("  %s%s%s  %s%s%s  %s",
		terminal.Bold, title, terminal.Reset,
		color, status, terminal.Reset,
		terminal.Dim+item.CommandLine+terminal.Reset,
	)

	ow.Println(header)

	if item.Skipped && item.SkipReason != "" {
		ow.Println(terminal.Dim + "    " + item.SkipReason + terminal.Reset)
	}

	if item.ErrText != "" {
		ow.Println(terminal.Red + "    " + item.ErrText + terminal.Reset)
	}

	if len(item.Counts) > 0 {
		ow.Println(terminal.Dim + "    counts: " + formatCounts(item.Counts) + terminal.Reset)
	}

	if item.Output != "" && !item.Skipped {
		body := truncateRunes(strings.TrimSpace(item.Output), auditOutputMaxRunes)

		for line := range strings.SplitSeq(body, "\n") {
			ow.Println(terminal.Gray + "    " + line + terminal.Reset)
		}
	}

	ow.Println("")
}

func formatCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}

	var parts []string

	for severity, count := range counts {
		parts = append(parts, fmt.Sprintf("%s=%d", severity, count))
	}

	return strings.Join(parts, ", ")
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)

	if len(runes) <= limit {
		return s
	}

	return string(runes[:limit]) + "\n… (truncated)"
}

func auditStatusFromReport(report result.AuditReport) (icon, color, text string) {
	if len(report.Items) == 0 {
		return terminal.WarningSign, terminal.Yellow, "No audits ran (no matching scopes or tools)"
	}

	allSkipped := true

	for _, item := range report.Items {
		if !item.Skipped {
			allSkipped = false

			break
		}
	}

	if allSkipped {
		return terminal.WarningSign, terminal.Yellow, "All audits skipped (missing manifests or optional tools)"
	}

	hasErr := false
	hasIssues := false

	for _, item := range report.Items {
		if item.Skipped {
			continue
		}

		if item.ErrText != "" {
			hasErr = true
		}

		if !item.OK {
			hasIssues = true
		}
	}

	switch {
	case hasErr:
		return terminal.CrossMark, terminal.Red, "Audit completed with errors (tool missing or failed to run)"
	case hasIssues:
		return terminal.WarningSign, terminal.Yellow, "Vulnerabilities or policy findings reported"
	default:
		return terminal.CheckMark, terminal.Green, "No blocking audit issues"
	}
}
