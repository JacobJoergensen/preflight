package render

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

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

	writeMarkdownAuditTable(&doc, report.Items)
	writeMarkdownAuditDetails(&doc, report.Items)

	doc.WriteString("\n---\n\n")

	_, err := r.Out.Write([]byte(doc.String()))
	return err
}

func markdownAuditStatus(report result.AuditReport) (symbol, text string) {
	if report.Canceled {
		return "⏸", "Audit canceled"
	}

	if len(report.Items) == 0 {
		return "⚠", "No audits ran (no matching scopes or tools)"
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

func writeMarkdownAuditTable(doc *strings.Builder, items []result.AuditItem) {
	if len(items) == 0 {
		return
	}

	hasOther := auditItemsHaveOtherSeverities(items)

	headers := []string{"Ecosystem", "Status"}
	separators := []string{"---------", "------"}

	for _, severity := range auditSeverityColumns {
		headers = append(headers, capitalize(severity))
		separators = append(separators, "---")
	}

	if hasOther {
		headers = append(headers, "Other")
		separators = append(separators, "---")
	}

	headers = append(headers, "Tool")
	separators = append(separators, "----")

	fmt.Fprintf(doc, "| %s |\n", strings.Join(headers, " | "))
	fmt.Fprintf(doc, "| %s |\n", strings.Join(separators, " | "))

	for _, item := range items {
		writeMarkdownAuditRow(doc, item, hasOther)
	}

	doc.WriteString("\n")
}

func writeMarkdownAuditRow(doc *strings.Builder, item result.AuditItem, includeOther bool) {
	row := []string{
		escapeMarkdownCell(auditItemTitle(item)),
		auditItemMarkdownStatus(item),
	}

	for _, severity := range auditSeverityColumns {
		row = append(row, strconv.Itoa(item.Counts[severity]))
	}

	if includeOther {
		row = append(row, formatOtherSeverities(item.Counts))
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

func auditItemsHaveOtherSeverities(items []result.AuditItem) bool {
	known := make(map[string]struct{}, len(auditSeverityColumns))

	for _, severity := range auditSeverityColumns {
		known[severity] = struct{}{}
	}

	for _, item := range items {
		for severity := range item.Counts {
			if _, ok := known[severity]; !ok && item.Counts[severity] > 0 {
				return true
			}
		}
	}

	return false
}

func formatOtherSeverities(counts map[string]int) string {
	known := make(map[string]struct{}, len(auditSeverityColumns))

	for _, severity := range auditSeverityColumns {
		known[severity] = struct{}{}
	}

	var extras []string

	for severity, count := range counts {
		if _, ok := known[severity]; ok {
			continue
		}

		if count == 0 {
			continue
		}

		extras = append(extras, fmt.Sprintf("%s=%d", severity, count))
	}

	if len(extras) == 0 {
		return "—"
	}

	sort.Strings(extras)

	return "`" + strings.Join(extras, ", ") + "`"
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
