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

	if len(report.Projects) > 0 {
		writeMarkdownCheckProjectSections(&doc, report)
	} else {
		writeMarkdownCheckTable(&doc, report.Items)
		writeMarkdownCheckIssues(&doc, report.Items)
		writeMarkdownCheckOutdated(&doc, report.Items)
	}

	doc.WriteString("\n---\n\n")

	_, err := r.Out.Write([]byte(doc.String()))
	return err
}

func writeMarkdownCheckProjectSections(doc *strings.Builder, report result.CheckReport) {
	itemsByProject := make(map[string][]result.CheckItem, len(report.Projects))

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

		writeMarkdownCheckTable(doc, items)
		writeMarkdownCheckIssues(doc, items)
		writeMarkdownCheckOutdated(doc, items)
	}
}

func markdownCheckStatus(report result.CheckReport) (symbol, text string) {
	if report.Canceled {
		return "⏸", "Checks canceled"
	}

	if len(report.Projects) > 0 {
		return markdownMonorepoCheckStatus(report)
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

func markdownMonorepoCheckStatus(report result.CheckReport) (symbol, text string) {
	projectsWithErrors := make(map[string]struct{})
	projectsWithWarnings := make(map[string]struct{})

	for _, item := range report.Items {
		if len(item.Errors) > 0 {
			projectsWithErrors[item.Project] = struct{}{}
		}

		if len(item.Warnings) > 0 {
			projectsWithWarnings[item.Project] = struct{}{}
		}
	}

	totalProjects := len(report.Projects)

	if len(projectsWithErrors) > 0 {
		return "✗", fmt.Sprintf("%d of %d project%s reported errors", len(projectsWithErrors), totalProjects, pluralSuffix(totalProjects))
	}

	if len(projectsWithWarnings) > 0 {
		return "⚠", fmt.Sprintf("%d of %d project%s reported warnings", len(projectsWithWarnings), totalProjects, pluralSuffix(totalProjects))
	}

	return "✓", fmt.Sprintf("%d project%s checked, all healthy", totalProjects, pluralSuffix(totalProjects))
}

func writeMarkdownCheckTable(doc *strings.Builder, items []result.CheckItem) {
	if len(items) == 0 {
		return
	}

	doc.WriteString("| Ecosystem | Status | Errors | Warnings | Outdated |\n")
	doc.WriteString("|-----------|--------|--------|----------|----------|\n")

	for _, item := range items {
		fmt.Fprintf(doc, "| %s | %s | %d | %d | %d |\n",
			escapeMarkdownCell(item.ScopeDisplay),
			checkItemStatus(item),
			len(item.Errors),
			len(item.Warnings),
			len(item.Outdated),
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

func writeMarkdownCheckIssues(doc *strings.Builder, items []result.CheckItem) {
	hasIssues := false

	for _, item := range items {
		if len(item.Errors) > 0 || len(item.Warnings) > 0 {
			hasIssues = true
			break
		}
	}

	if !hasIssues {
		return
	}

	doc.WriteString("#### Issues\n\n")

	for _, item := range items {
		if len(item.Errors) == 0 && len(item.Warnings) == 0 {
			continue
		}

		fmt.Fprintf(doc, "**%s**\n\n", escapeMarkdownCell(item.ScopeDisplay))

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

func writeMarkdownCheckOutdated(doc *strings.Builder, items []result.CheckItem) {
	hasOutdated := false

	for _, item := range items {
		if len(item.Outdated) > 0 {
			hasOutdated = true
			break
		}
	}

	if !hasOutdated {
		return
	}

	doc.WriteString("#### Outdated packages\n\n")

	for _, item := range items {
		if len(item.Outdated) == 0 {
			continue
		}

		fmt.Fprintf(doc, "**%s**\n\n", escapeMarkdownCell(item.ScopeDisplay))

		for _, pkg := range item.Outdated {
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
