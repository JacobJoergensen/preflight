package render

import (
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
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

	ow.PrintNewLines(1)

	renderAuditItemsGroupedByProject(ow, report)

	icon, color, text := auditStatusFromReport(report)
	renderStatusFooter(ow, footerStatus{Icon: icon, Color: color, Text: text}, []footerMetadataLine{endedFooterLine(report.EndedAt)})

	return nil
}

func renderAuditItemsGroupedByProject(ow *terminal.OutputWriter, report result.AuditReport) {
	renderByProject(ow, report.Projects, report.Items,
		func(p result.Project) string { return p.RelativePath },
		func(i result.AuditItem) string { return i.Project },
		renderProjectHeader,
		renderAuditCardTTY,
	)
}

func renderAuditCardTTY(ow *terminal.OutputWriter, item result.AuditItem) {
	title := item.ScopeDisplay

	if title == "" {
		title = item.ScopeID
	}

	color := terminal.Green
	status := "PASS"

	switch {
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

	if item.ErrText != "" {
		ow.Println(terminal.Red + "    " + item.ErrText + terminal.Reset)
	}

	counts := ecosystem.CountsBySeverity(item.Findings)

	if len(counts) > 0 {
		ow.Println(terminal.Dim + "    counts: " + formatCounts(counts) + terminal.Reset)
	}

	for _, finding := range item.Findings {
		ow.Println(formatFinding(finding))
	}

	if len(item.Findings) == 0 && item.Output != "" {
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

	for _, severity := range auditSeverityColumns {
		if count := counts[severity]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", severity, count))
		}
	}

	return strings.Join(parts, ", ")
}

func formatFinding(finding model.Finding) string {
	severity := ecosystem.NormalizeSeverity(finding.Severity)
	tag := severityColor(severity) + strings.ToUpper(severity) + terminal.Reset

	detail := finding.ID

	if finding.Package != "" {
		pkg := finding.Package

		if finding.Version != "" {
			pkg += " " + finding.Version
		}

		if detail == "" {
			detail = pkg
		} else {
			detail += "  " + pkg
		}
	}

	if finding.FixedIn != "" {
		detail += "  (fixed in " + finding.FixedIn + ")"
	}

	return "    " + tag + "  " + terminal.Dim + strings.TrimSpace(detail) + terminal.Reset
}

func severityColor(severity string) string {
	switch severity {
	case "critical", "high":
		return terminal.Red
	case "moderate":
		return terminal.Yellow
	default:
		return terminal.Dim
	}
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

	if len(report.Projects) > 0 {
		return monorepoAuditStatusFromReport(report)
	}

	hasErr := false
	hasIssues := false

	for _, item := range report.Items {
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

func monorepoAuditStatusFromReport(report result.AuditReport) (icon, color, text string) {
	failedProjects := countProjects(report.Items, func(i result.AuditItem) (string, bool) { return i.Project, i.ErrText != "" })
	issueProjects := countProjects(report.Items, func(i result.AuditItem) (string, bool) { return i.Project, !i.OK })

	totalProjects := len(report.Projects)

	if failedProjects > 0 {
		return terminal.CrossMark, terminal.Red, projectStatusLine(failedProjects, totalProjects, "failed to audit")
	}

	if issueProjects > 0 {
		return terminal.WarningSign, terminal.Yellow, projectStatusLine(issueProjects, totalProjects, "reported vulnerabilities")
	}

	return terminal.CheckMark, terminal.Green, fmt.Sprintf("%d project%s audited, no blocking issues", totalProjects, pluralSuffix(totalProjects))
}
