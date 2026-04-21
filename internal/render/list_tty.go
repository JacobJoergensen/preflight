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

	if len(report.Dependencies) == 0 {
		ow.Println(terminal.Dim + "  " + terminal.WarningSign + " No package managers returned dependencies to list." + terminal.Reset)
	} else {
		for _, id := range report.AdapterIDs {
			deps, ok := report.Dependencies[id]

			if !ok {
				continue
			}

			var outdated []adapter.OutdatedPackage
			if report.Outdated != nil {
				outdated = report.Outdated[id]
			}

			elapsedMillis := int64(0)
			if report.Elapsed != nil {
				elapsedMillis = report.Elapsed[id]
			}

			renderListScopeCardTTY(ow, listScopeDisplay(report.Displays, id), deps, outdated, elapsedMillis)
		}
	}

	icon, color, text := listStatusFromReport(report)
	renderStatusFooter(ow, footerStatus{Icon: icon, Color: color, Text: text}, []footerMetadataLine{endedFooterLine(report.EndedAt)})

	return nil
}

func listScopeDisplay(displays map[string]string, adapterID string) string {
	if name, ok := displays[adapterID]; ok && name != "" {
		return name
	}

	if adapterID == "" {
		return adapterID
	}

	return strings.ToUpper(adapterID[:1]) + adapterID[1:]
}

func renderListQuiet(ow *terminal.OutputWriter, report result.DependencyReport) error {
	if report.Outdated == nil {
		return nil
	}

	for _, id := range report.AdapterIDs {
		outdated := report.Outdated[id]

		if len(outdated) == 0 {
			continue
		}

		header := listScopeDisplay(report.Displays, id) + "  " + terminal.Yellow + terminal.Lightning + " " + fmt.Sprintf("%d outdated", len(outdated)) + terminal.Reset
		ow.Println(header)

		printOutdatedLinesQuietTTY(ow, outdated)
	}

	return nil
}

func renderListScopeCardTTY(ow *terminal.OutputWriter, scopeDisplay string, deps []string, outdated []adapter.OutdatedPackage, elapsedMillis int64) {
	ow.PrintNewLines(1)

	badge := terminal.Green + terminal.Bold + "OK" + terminal.Reset

	if len(deps) == 0 {
		badge = terminal.Yellow + terminal.Bold + "EMPTY" + terminal.Reset
	}

	elapsed := ""

	if elapsedMillis > 0 {
		elapsed = terminal.Dim + fmt.Sprintf(" %dms", elapsedMillis) + terminal.Reset
	}

	outdatedIndicator := ""

	if len(outdated) > 0 {
		outdatedIndicator = "  " + terminal.Yellow + terminal.Lightning + " " + fmt.Sprintf("%d outdated", len(outdated)) + terminal.Reset
	}

	header := fmt.Sprintf("  %s%s%s  %s%s%s",
		terminal.Bold, scopeDisplay, terminal.Reset,
		badge, elapsed, outdatedIndicator,
	)

	ow.Println(header)

	if len(deps) > 0 {
		ow.Println(terminal.Dim + "  " + listDepCountSummary(len(deps)) + terminal.Reset)
	} else {
		ow.Println(terminal.Dim + "  No dependencies in this scope" + terminal.Reset)
	}

	ow.Println(terminal.Gray + strings.Repeat("─", checkCardRuleWidth) + terminal.Reset)

	ow.Println(terminal.Dim + "  Dependencies" + terminal.Reset)

	if len(deps) == 0 {
		ow.Printf("%s%s%s %s%s\n",
			terminal.Red, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CrossMark,
			"No dependencies found.", terminal.Reset,
		)

		return
	}

	outdatedMap := make(map[string]adapter.OutdatedPackage)
	for _, pkg := range outdated {
		outdatedMap[pkg.Name] = pkg
	}

	for _, dep := range deps {
		if pkg, isOutdated := outdatedMap[dep]; isOutdated {
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
	managerCount := 0
	totalDeps := 0

	for _, adapterID := range report.AdapterIDs {
		deps, ok := report.Dependencies[adapterID]

		if !ok {
			continue
		}

		managerCount++
		totalDeps += len(deps)
	}

	if managerCount == 0 {
		return terminal.WarningSign, terminal.Yellow, "Nothing to list."
	}

	line := "Listed " + listDepPhrase(totalDeps) + " across " + listManagerPhrase(managerCount) + "."

	return terminal.CheckMark, terminal.Green, line
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
