package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
)

type MarkdownLicenseRenderer struct {
	Out              io.Writer
	PolicyConfigured bool
}

func (r MarkdownLicenseRenderer) Render(report result.LicenseReport) error {
	var doc strings.Builder

	doc.WriteString("## 📜 License Compliance\n\n")

	symbol, text := markdownLicenseStatus(report, r.PolicyConfigured)
	fmt.Fprintf(&doc, "**Status:** %s %s\n\n", symbol, text)

	if len(report.Projects) > 0 {
		writeMarkdownLicenseProjectSections(&doc, report)
	} else {
		writeMarkdownLicenseTable(&doc, report.Items)
		writeMarkdownLicenseViolations(&doc, report.Items)
	}

	doc.WriteString("\n---\n\n")

	_, err := r.Out.Write([]byte(doc.String()))

	return err
}

func writeMarkdownLicenseProjectSections(doc *strings.Builder, report result.LicenseReport) {
	itemsByProject := make(map[string][]result.LicenseItem, len(report.Projects))

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
		writeMarkdownLicenseTable(doc, items)
		writeMarkdownLicenseViolations(doc, items)
	}
}

func markdownLicenseStatus(report result.LicenseReport, policyConfigured bool) (symbol, text string) {
	if report.Canceled {
		return "⏸", "License check canceled"
	}

	if len(report.Items) == 0 {
		return "⚠", "No license data (no matching scopes or tools)"
	}

	if len(report.Projects) > 0 {
		return markdownMonorepoLicenseStatus(report, policyConfigured)
	}

	var hasErr, hasViolations bool

	for _, item := range report.Items {
		if item.ErrText != "" {
			hasErr = true
		}

		if len(item.Violations) > 0 {
			hasViolations = true
		}
	}

	switch {
	case hasErr:
		return "✗", "License check completed with errors"
	case hasViolations:
		return "✗", "License policy violations found"
	case !policyConfigured:
		return "⚠", noLicensePolicyText
	}

	return "✓", "All licenses comply with policy"
}

func markdownMonorepoLicenseStatus(report result.LicenseReport, policyConfigured bool) (symbol, text string) {
	failedProjects := countProjects(report.Items, func(i result.LicenseItem) (string, bool) { return i.Project, i.ErrText != "" })
	violationProjects := countProjects(report.Items, func(i result.LicenseItem) (string, bool) { return i.Project, len(i.Violations) > 0 })

	totalProjects := len(report.Projects)

	if failedProjects > 0 {
		return "✗", fmt.Sprintf("%d of %d project%s failed the license check", failedProjects, totalProjects, pluralSuffix(totalProjects))
	}

	if violationProjects > 0 {
		return "✗", fmt.Sprintf("%d of %d project%s have license violations", violationProjects, totalProjects, pluralSuffix(totalProjects))
	}

	if !policyConfigured {
		return "⚠", noLicensePolicyText
	}

	return "✓", fmt.Sprintf("%d project%s checked, all licenses comply", totalProjects, pluralSuffix(totalProjects))
}

func writeMarkdownLicenseTable(doc *strings.Builder, items []result.LicenseItem) {
	if len(items) == 0 {
		return
	}

	doc.WriteString("| Ecosystem | Status | Inspected | Violations |\n")
	doc.WriteString("| --------- | ------ | --------- | ---------- |\n")

	for _, item := range items {
		fmt.Fprintf(doc, "| %s | %s | %d | %d |\n",
			escapeMarkdownCell(licenseItemTitle(item)),
			licenseItemMarkdownStatus(item),
			item.Inspected,
			len(item.Violations),
		)
	}

	doc.WriteString("\n")
}

func licenseItemTitle(item result.LicenseItem) string {
	if item.ScopeDisplay != "" {
		return item.ScopeDisplay
	}

	return item.ScopeID
}

func licenseItemMarkdownStatus(item result.LicenseItem) string {
	switch {
	case item.ErrText != "":
		return "✗ ERROR"
	case len(item.Violations) > 0:
		return "✗ VIOLATIONS"
	}

	return "✓ PASS"
}

func writeMarkdownLicenseViolations(doc *strings.Builder, items []result.LicenseItem) {
	for _, item := range items {
		if len(item.Violations) == 0 {
			continue
		}

		fmt.Fprintf(doc, "<details><summary>%s violations</summary>\n\n", escapeMarkdownCell(licenseItemTitle(item)))
		doc.WriteString("| Package | Version | License | Reason |\n")
		doc.WriteString("| ------- | ------- | ------- | ------ |\n")

		for _, violation := range item.Violations {
			fmt.Fprintf(doc, "| %s | %s | %s | %s |\n",
				escapeMarkdownCell(violation.Package),
				escapeMarkdownCell(violation.Version),
				escapeMarkdownCell(violation.License),
				escapeMarkdownCell(violation.Reason),
			)
		}

		doc.WriteString("\n</details>\n\n")
	}
}
