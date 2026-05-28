package render

import (
	"fmt"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const noLicensePolicyText = "No license policy configured (set licenses.allow or licenses.deny)"

type TTYLicenseRenderer struct {
	Out              *terminal.OutputWriter
	PolicyConfigured bool
}

func (r TTYLicenseRenderer) Render(report result.LicenseReport) error {
	ow := r.Out

	if ow == nil {
		ow = terminal.NewOutputWriter()
	}

	if terminal.Quiet {
		return nil
	}

	ow.PrintNewLines(1)

	renderLicenseItemsGroupedByProject(ow, report)

	icon, color, text := licenseStatusFromReport(report, r.PolicyConfigured)
	renderStatusFooter(ow, footerStatus{Icon: icon, Color: color, Text: text}, []footerMetadataLine{endedFooterLine(report.EndedAt)})

	return nil
}

func renderLicenseItemsGroupedByProject(ow *terminal.OutputWriter, report result.LicenseReport) {
	renderByProject(ow, report.Projects, report.Items,
		func(p result.Project) string { return p.RelativePath },
		func(i result.LicenseItem) string { return i.Project },
		renderProjectHeader,
		renderLicenseCardTTY,
	)
}

func renderLicenseCardTTY(ow *terminal.OutputWriter, item result.LicenseItem) {
	title := item.ScopeDisplay

	if title == "" {
		title = item.ScopeID
	}

	color := terminal.Green
	status := "PASS"

	switch {
	case item.ErrText != "":
		color, status = terminal.Red, "ERROR"
	case len(item.Violations) > 0:
		color, status = terminal.Red, "VIOLATIONS"
	}

	inspected := fmt.Sprintf("%d package%s inspected", item.Inspected, pluralSuffix(item.Inspected))

	ow.Println(fmt.Sprintf("  %s%s%s  %s%s%s  %s",
		terminal.Bold, title, terminal.Reset,
		color, status, terminal.Reset,
		terminal.Dim+inspected+terminal.Reset,
	))

	if item.ErrText != "" {
		ow.Println(terminal.Red + "    " + item.ErrText + terminal.Reset)
	}

	for _, violation := range item.Violations {
		pkg := violation.Package

		if violation.Version != "" {
			pkg += "@" + violation.Version
		}

		ow.Println("    " + terminal.Red + violation.License + terminal.Reset + "  " + terminal.Dim + pkg + ": " + violation.Reason + terminal.Reset)
	}

	ow.Println("")
}

func licenseStatusFromReport(report result.LicenseReport, policyConfigured bool) (icon, color, text string) {
	if len(report.Items) == 0 {
		return terminal.WarningSign, terminal.Yellow, "No license data (no matching scopes or tools)"
	}

	if len(report.Projects) > 0 {
		return monorepoLicenseStatusFromReport(report, policyConfigured)
	}

	hasErr := false
	hasViolations := false

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
		return terminal.CrossMark, terminal.Red, "License check completed with errors"
	case hasViolations:
		return terminal.CrossMark, terminal.Red, "License policy violations found"
	case !policyConfigured:
		return terminal.WarningSign, terminal.Yellow, noLicensePolicyText
	default:
		return terminal.CheckMark, terminal.Green, "All licenses comply with policy"
	}
}

func monorepoLicenseStatusFromReport(report result.LicenseReport, policyConfigured bool) (icon, color, text string) {
	failedProjects := countProjects(report.Items, func(i result.LicenseItem) (string, bool) { return i.Project, i.ErrText != "" })
	violationProjects := countProjects(report.Items, func(i result.LicenseItem) (string, bool) { return i.Project, len(i.Violations) > 0 })

	totalProjects := len(report.Projects)

	if failedProjects > 0 {
		return terminal.CrossMark, terminal.Red, projectStatusLine(failedProjects, totalProjects, "failed the license check")
	}

	if violationProjects > 0 {
		return terminal.CrossMark, terminal.Red, projectStatusLine(violationProjects, totalProjects, "have license violations")
	}

	if !policyConfigured {
		return terminal.WarningSign, terminal.Yellow, noLicensePolicyText
	}

	return terminal.CheckMark, terminal.Green, fmt.Sprintf("%d project%s checked, all licenses comply", totalProjects, pluralSuffix(totalProjects))
}
