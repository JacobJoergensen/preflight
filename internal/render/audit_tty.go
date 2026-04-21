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

	ow.PrintNewLines(1)

	renderAuditItemsGroupedByProject(ow, report)

	icon, color, text := auditStatusFromReport(report)
	renderStatusFooter(ow, footerStatus{Icon: icon, Color: color, Text: text}, []footerMetadataLine{endedFooterLine(report.EndedAt)})

	return nil
}

func renderAuditItemsGroupedByProject(ow *terminal.OutputWriter, report result.AuditReport) {
	if len(report.Projects) == 0 {
		for _, item := range report.Items {
			renderAuditCardTTY(ow, item)
		}

		return
	}

	itemsByProject := make(map[string][]result.AuditItem, len(report.Projects))

	for _, item := range report.Items {
		itemsByProject[item.Project] = append(itemsByProject[item.Project], item)
	}

	for i, project := range report.Projects {
		items := itemsByProject[project.RelativePath]

		if len(items) == 0 {
			continue
		}

		if i > 0 {
			ow.PrintNewLines(1)
		}

		renderAuditProjectHeader(ow, project)

		for _, item := range items {
			renderAuditCardTTY(ow, item)
		}
	}
}

func renderAuditProjectHeader(ow *terminal.OutputWriter, project result.AuditProject) {
	line := "  " + terminal.Bold + terminal.Cyan + project.RelativePath + terminal.Reset

	if project.Name != "" {
		line += "  " + terminal.Dim + project.Name + terminal.Reset
	}

	ow.Println(line)
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

	if len(item.Counts) > 0 {
		ow.Println(terminal.Dim + "    counts: " + formatCounts(item.Counts) + terminal.Reset)
	}

	if item.Output != "" {
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
	projectsWithErr := make(map[string]struct{})
	projectsWithIssues := make(map[string]struct{})

	for _, item := range report.Items {
		if item.ErrText != "" {
			projectsWithErr[item.Project] = struct{}{}
		}

		if !item.OK {
			projectsWithIssues[item.Project] = struct{}{}
		}
	}

	totalProjects := len(report.Projects)

	if len(projectsWithErr) > 0 {
		return terminal.CrossMark, terminal.Red, fmt.Sprintf("%d of %d project%s failed to audit", len(projectsWithErr), totalProjects, pluralSuffix(totalProjects))
	}

	if len(projectsWithIssues) > 0 {
		return terminal.WarningSign, terminal.Yellow, fmt.Sprintf("%d of %d project%s reported vulnerabilities", len(projectsWithIssues), totalProjects, pluralSuffix(totalProjects))
	}

	return terminal.CheckMark, terminal.Green, fmt.Sprintf("%d project%s audited, no blocking issues", totalProjects, pluralSuffix(totalProjects))
}
