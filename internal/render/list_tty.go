package render

import (
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type TTYListRenderer struct {
	Out *terminal.OutputWriter
}

func (r TTYListRenderer) Render(report result.DependencyReport) error {
	ow := r.Out

	if ow == nil {
		ow = terminal.NewOutputWriter()
	}

	if terminal.Quiet {
		return renderListQuiet(ow, report)
	}

	ow.PrintNewLines(1)

	if len(report.Items) == 0 {
		ow.Println(terminal.Dim + "  " + terminal.WarningSign + " No package managers returned dependencies to list." + terminal.Reset)
	} else {
		renderListItemsGroupedByProject(ow, report)
	}

	icon, color, text := listStatusFromReport(report)
	renderStatusFooter(ow, footerStatus{Icon: icon, Color: color, Text: text}, []footerMetadataLine{endedFooterLine(report.EndedAt)})

	return nil
}

func renderListItemsGroupedByProject(ow *terminal.OutputWriter, report result.DependencyReport) {
	if len(report.Projects) == 0 {
		for _, item := range report.Items {
			renderListItemCardTTY(ow, item)
		}

		return
	}

	itemsByProject := make(map[string][]result.DependencyItem, len(report.Projects))

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

		renderListProjectHeader(ow, project)

		for _, item := range items {
			renderListItemCardTTY(ow, item)
		}
	}
}

func renderListProjectHeader(ow *terminal.OutputWriter, project result.DependencyProject) {
	line := "  " + terminal.Bold + terminal.Cyan + project.RelativePath + terminal.Reset

	if project.Name != "" {
		line += "  " + terminal.Dim + project.Name + terminal.Reset
	}

	ow.Println(line)
}

func renderListQuiet(ow *terminal.OutputWriter, report result.DependencyReport) error {
	for _, item := range report.Items {
		if len(item.Outdated) == 0 {
			continue
		}

		header := item.Display + "  " + terminal.Yellow + terminal.Lightning + " " + fmt.Sprintf("%d outdated", len(item.Outdated)) + terminal.Reset
		ow.Println(header)

		printOutdatedLinesQuietTTY(ow, item.Outdated)
	}

	return nil
}

func renderListItemCardTTY(ow *terminal.OutputWriter, item result.DependencyItem) {
	ow.PrintNewLines(1)

	badge := terminal.Green + terminal.Bold + "OK" + terminal.Reset

	if len(item.Dependencies) == 0 {
		badge = terminal.Yellow + terminal.Bold + "EMPTY" + terminal.Reset
	}

	elapsed := ""

	if item.ElapsedMillis > 0 {
		elapsed = terminal.Dim + fmt.Sprintf(" %dms", item.ElapsedMillis) + terminal.Reset
	}

	outdatedIndicator := ""

	if len(item.Outdated) > 0 {
		outdatedIndicator = "  " + terminal.Yellow + terminal.Lightning + " " + fmt.Sprintf("%d outdated", len(item.Outdated)) + terminal.Reset
	}

	header := fmt.Sprintf("  %s%s%s  %s%s%s",
		terminal.Bold, item.Display, terminal.Reset,
		badge, elapsed, outdatedIndicator,
	)

	ow.Println(header)

	if len(item.Dependencies) > 0 {
		ow.Println(terminal.Dim + "  " + listDepCountSummary(len(item.Dependencies)) + terminal.Reset)
	} else {
		ow.Println(terminal.Dim + "  No dependencies in this scope" + terminal.Reset)
	}

	ow.Println(terminal.Gray + strings.Repeat("─", checkCardRuleWidth) + terminal.Reset)

	ow.Println(terminal.Dim + "  Dependencies" + terminal.Reset)

	if len(item.Dependencies) == 0 {
		ow.Printf("%s%s%s %s%s\n",
			terminal.Red, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CrossMark,
			"No dependencies found.", terminal.Reset,
		)

		return
	}

	outdatedByName := make(map[string]adapter.OutdatedPackage, len(item.Outdated))

	for _, pkg := range item.Outdated {
		outdatedByName[pkg.Name] = pkg
	}

	for _, dep := range item.Dependencies {
		if pkg, isOutdated := outdatedByName[dep]; isOutdated {
			ow.Printf("%s%s%s %s %s%s%s → %s%s%s\n",
				terminal.Yellow, strings.Repeat(" ", ttyProjectBodySpaces), terminal.Lightning,
				dep,
				terminal.Dim, pkg.Current, terminal.Reset,
				terminal.Green, pkg.Latest, terminal.Reset,
			)
		} else {
			ow.Printf("%s%s%s %s%s\n",
				terminal.Green, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CheckMark,
				dep, terminal.Reset,
			)
		}
	}
}

func listDepCountSummary(count int) string {
	if count == 1 {
		return "1 required dependency"
	}

	return fmt.Sprintf("%d required dependencies", count)
}

func listStatusFromReport(report result.DependencyReport) (icon string, color string, text string) {
	if len(report.Items) == 0 {
		return terminal.WarningSign, terminal.Yellow, "Nothing to list."
	}

	if len(report.Projects) > 0 {
		return monorepoListStatusFromReport(report)
	}

	totalDeps := 0

	for _, item := range report.Items {
		totalDeps += len(item.Dependencies)
	}

	line := "Listed " + listDepPhrase(totalDeps) + " across " + listManagerPhrase(len(report.Items)) + "."

	return terminal.CheckMark, terminal.Green, line
}

func monorepoListStatusFromReport(report result.DependencyReport) (icon string, color string, text string) {
	totalDeps := 0
	managersWithDeps := 0

	for _, item := range report.Items {
		if len(item.Dependencies) == 0 {
			continue
		}

		totalDeps += len(item.Dependencies)
		managersWithDeps++
	}

	return terminal.CheckMark, terminal.Green, fmt.Sprintf("Listed %s across %s in %d project%s",
		listDepPhrase(totalDeps),
		listManagerPhrase(managersWithDeps),
		len(report.Projects),
		pluralSuffix(len(report.Projects)),
	)
}

func listDepPhrase(count int) string {
	if count == 1 {
		return "1 dependency"
	}

	return fmt.Sprintf("%d dependencies", count)
}

func listManagerPhrase(count int) string {
	if count == 1 {
		return "1 package manager"
	}

	return fmt.Sprintf("%d package managers", count)
}
