package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
)

type MarkdownCheckRenderer struct {
	Out io.Writer
}

func (r MarkdownCheckRenderer) Render(report result.CheckReport) error {
	var doc strings.Builder

	doc.WriteString("## 🚀 PreFlight Check\n\n")

	symbol, text := markdownCheckStatus(report)
	fmt.Fprintf(&doc, "**Status:** %s %s\n\n", symbol, text)

	writeMarkdownCheckTable(&doc, report)
	writeMarkdownCheckIssues(&doc, report)
	writeMarkdownCheckOutdated(&doc, report)

	doc.WriteString("\n---\n\n")

	_, err := r.Out.Write([]byte(doc.String()))
	return err
}

func markdownCheckStatus(report result.CheckReport) (symbol, text string) {
	if report.Canceled {
		return "⏸", "Checks canceled"
	}

	var totalErrors, totalWarnings int

	for _, item := range report.Items {
		totalErrors += len(item.Errors)
		totalWarnings += len(item.Warnings)
	}

	switch {
	case totalErrors > 0:
		return "✗", "Check completed, please resolve"
	case totalWarnings > 0:
		return "⚠", "Check completed with warnings, please review"
	}

	return "✓", "Check completed successfully"
}

func writeMarkdownCheckTable(doc *strings.Builder, report result.CheckReport) {
	if len(report.Items) == 0 {
		return
	}

	doc.WriteString("| Ecosystem | Status | Errors | Warnings | Outdated |\n")
	doc.WriteString("|-----------|--------|--------|----------|----------|\n")

	for _, item := range report.Items {
		status := checkItemStatus(item)
		outdatedCount := len(report.Outdated[item.ScopeID])

		fmt.Fprintf(doc, "| %s | %s | %d | %d | %d |\n",
			escapeMarkdownCell(item.ScopeDisplay),
			status,
			len(item.Errors),
			len(item.Warnings),
			outdatedCount,
		)
	}

	doc.WriteString("\n")
}

func checkItemStatus(item result.CheckItem) string {
	switch {
	case len(item.Errors) > 0:
		return "✗ FAIL"
	case len(item.Warnings) > 0:
		return "⚠ WARN"
	}

	return "✓ OK"
}

func writeMarkdownCheckIssues(doc *strings.Builder, report result.CheckReport) {
	hasIssues := false

	for _, item := range report.Items {
		if len(item.Errors) > 0 || len(item.Warnings) > 0 {
			hasIssues = true
			break
		}
	}

	if !hasIssues {
		return
	}

	doc.WriteString("### Issues\n\n")

	for _, item := range report.Items {
		if len(item.Errors) == 0 && len(item.Warnings) == 0 {
			continue
		}

		fmt.Fprintf(doc, "#### %s\n\n", escapeMarkdownCell(item.ScopeDisplay))

		writeMarkdownMessageList(doc, item.Errors, "✗")
		writeMarkdownMessageList(doc, item.Warnings, "⚠")

		doc.WriteString("\n")
	}
}

func writeMarkdownMessageList(doc *strings.Builder, messages []model.Message, symbol string) {
	for _, msg := range messages {
		fmt.Fprintf(doc, "- %s %s\n", symbol, escapeMarkdownCell(msg.Text))
	}
}

func writeMarkdownCheckOutdated(doc *strings.Builder, report result.CheckReport) {
	if len(report.Outdated) == 0 {
		return
	}

	hasOutdated := false

	for _, packages := range report.Outdated {
		if len(packages) > 0 {
			hasOutdated = true
			break
		}
	}

	if !hasOutdated {
		return
	}

	doc.WriteString("### Outdated packages\n\n")

	for _, item := range report.Items {
		packages := report.Outdated[item.ScopeID]

		if len(packages) == 0 {
			continue
		}

		fmt.Fprintf(doc, "#### %s\n\n", escapeMarkdownCell(item.ScopeDisplay))

		for _, pkg := range packages {
			writeMarkdownOutdatedLine(doc, pkg)
		}

		doc.WriteString("\n")
	}
}

func writeMarkdownOutdatedLine(doc *strings.Builder, pkg adapter.OutdatedPackage) {
	fmt.Fprintf(doc, "- `%s` `%s` → `%s`\n",
		escapeMarkdownCell(pkg.Name),
		pkg.Current,
		pkg.Latest,
	)
}
