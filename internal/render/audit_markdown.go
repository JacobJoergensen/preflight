package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
)

var auditSeverityColumns = []string{"critical", "high", "moderate", "low", "info"}

type MarkdownAuditRenderer struct {
	Out io.Writer
}

func (r MarkdownAuditRenderer) Render(report result.AuditReport) error {
	var doc strings.Builder

	doc.WriteString("## 🛡️ Security Audit\n\n")

	symbol, text := markdownAuditStatus(report)
	fmt.Fprintf(&doc, "**Status:** %s %s\n\n", symbol, text)

	if len(report.Projects) > 0 {
		writeMarkdownAuditProjectSections(&doc, report)
	} else {
		writeMarkdownAuditTable(&doc, report.Items)
		writeMarkdownAuditDetails(&doc, report.Items)
	}

	doc.WriteString("\n---\n\n")

	_, err := r.Out.Write([]byte(doc.String()))
	return err
}

func writeMarkdownAuditProjectSections(doc *strings.Builder, report result.AuditReport) {
	itemsByProject := make(map[string][]result.AuditItem, len(report.Projects))

	for _, item := range report.Items {
		itemsByProject[item.Project] = append(itemsByProject[item.Project], item)
	}

	for _, project := range report.Projects {
		items := itemsByProject[project.RelativePath]

		if len(items) == 0 {
			continue
		}

		heading := "### " + project.RelativePath

		if project.Name != "" {
			heading += " · `" + project.Name + "`"
		}

		doc.WriteString(heading + "\n\n")

		writeMarkdownAuditTable(doc, items)
		writeMarkdownAuditDetails(doc, items)
	}
}

func markdownAuditStatus(report result.AuditReport) (symbol, text string) {
	if report.Canceled {
		return "⏸", "Audit canceled"
	}

	if len(report.Items) == 0 {
		return "⚠", "No audits ran (no matching scopes or tools)"
	}

	if len(report.Projects) > 0 {
		return markdownMonorepoAuditStatus(report)
	}

	var hasErr, hasIssues bool

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
		return "✗", "Audit completed with errors (tool missing or failed to run)"
	case hasIssues:
		return "⚠", "Vulnerabilities or policy findings reported"
	}

	return "✓", "No blocking audit issues"
}

func markdownMonorepoAuditStatus(report result.AuditReport) (symbol, text string) {
	failedProjects := countProjects(report.Items, func(i result.AuditItem) (string, bool) { return i.Project, i.ErrText != "" })
	issueProjects := countProjects(report.Items, func(i result.AuditItem) (string, bool) { return i.Project, !i.OK })

	totalProjects := len(report.Projects)

	if failedProjects > 0 {
		return "✗", fmt.Sprintf("%d of %d project%s failed to audit", failedProjects, totalProjects, pluralSuffix(totalProjects))
	}

	if issueProjects > 0 {
		return "⚠", fmt.Sprintf("%d of %d project%s reported vulnerabilities", issueProjects, totalProjects, pluralSuffix(totalProjects))
	}

	return "✓", fmt.Sprintf("%d project%s audited, no blocking issues", totalProjects, pluralSuffix(totalProjects))
}

func writeMarkdownAuditTable(doc *strings.Builder, items []result.AuditItem) {
	if len(items) == 0 {
		return
	}

	headers := []string{"Ecosystem", "Status"}
	separators := []string{"---------", "------"}

	for _, severity := range auditSeverityColumns {
		headers = append(headers, capitalize(severity))
		separators = append(separators, "---")
	}

	headers = append(headers, "Tool")
	separators = append(separators, "----")

	fmt.Fprintf(doc, "| %s |\n", strings.Join(headers, " | "))
	fmt.Fprintf(doc, "| %s |\n", strings.Join(separators, " | "))

	for _, item := range items {
		writeMarkdownAuditRow(doc, item)
	}

	doc.WriteString("\n")
}

func writeMarkdownAuditRow(doc *strings.Builder, item result.AuditItem) {
	row := make([]string, 0, 2+len(auditSeverityColumns)+1)
	row = append(row, escapeMarkdownCell(auditItemTitle(item)), auditItemMarkdownStatus(item))

	counts := ecosystem.CountsBySeverity(item.Findings)

	for _, severity := range auditSeverityColumns {
		row = append(row, strconv.Itoa(counts[severity]))
	}

	tool := item.CommandLine

	if tool == "" {
		tool = "—"
	} else {
		tool = "`" + tool + "`"
	}

	row = append(row, tool)

	fmt.Fprintf(doc, "| %s |\n", strings.Join(row, " | "))
}

func auditItemTitle(item result.AuditItem) string {
	if item.ScopeDisplay != "" {
		return item.ScopeDisplay
	}

	return item.ScopeID
}

func auditItemMarkdownStatus(item result.AuditItem) string {
	switch {
	case item.ErrText != "":
		return "✗ ERROR"
	case !item.OK:
		return "⚠ ISSUES"
	}

	return "✓ PASS"
}

func writeMarkdownAuditDetails(doc *strings.Builder, items []result.AuditItem) {
	for _, item := range items {
		body := strings.TrimSpace(item.Output)

		if body == "" && item.ErrText != "" {
			body = strings.TrimSpace(item.ErrText)
		}

		if body == "" {
			continue
		}

		if item.OK && item.ErrText == "" {
			continue
		}

		fmt.Fprintf(doc, "<details><summary>%s output</summary>\n\n", escapeMarkdownCell(auditItemTitle(item)))
		doc.WriteString("```\n")
		doc.WriteString(body)
		doc.WriteString("\n```\n\n</details>\n\n")
	}
}

func capitalize(word string) string {
	if word == "" {
		return ""
	}

	return strings.ToUpper(word[:1]) + word[1:]
}
