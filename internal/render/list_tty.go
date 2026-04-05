package render

import (
	"fmt"
	"strings"
	"time"

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
		return nil
	}

	ow.Println(terminal.Bold + terminal.Blue + "\n╭─────────────────────────────────────────╮" + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "│" + terminal.Cyan + terminal.Bold + "  " + terminal.Rocket + " PreFlight List  " + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "╰─────────────────────────────────────────╯" + terminal.Reset)
	ow.Println(terminal.Bold + "\nListing dependencies..." + terminal.Reset)

	ow.PrintNewLines(1)

	if len(report.Dependencies) == 0 {
		ow.Println(terminal.Dim + "  " + terminal.WarningSign + " No package managers returned dependencies to list." + terminal.Reset)
	} else {
		for _, id := range report.AdapterIDs {
			deps, ok := report.Dependencies[id]

			if !ok {
				continue
			}

			renderListScopeCardTTY(ow, listScopeDisplay(id), deps)
		}
	}

	statusIcon, statusColor, statusText := listStatusFromReport(report)

	endedAt := report.EndedAt

	if endedAt.IsZero() {
		endedAt = time.Now()
	}

	ow.Println(terminal.Bold + terminal.Blue + "\n╭────────────────────────────────────────────────────────────────╮" + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "│ " + statusColor + statusIcon + " Status: " + statusText + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "│ " + terminal.Dim + terminal.Clock + " Ended: " + endedAt.Format("02-01-2006 15:04:05") + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "╰────────────────────────────────────────────────────────────────╯" + terminal.Reset)

	return nil
}

func listScopeDisplay(adapterID string) string {
	if adapterID == "" {
		return adapterID
	}

	return strings.ToUpper(adapterID[:1]) + adapterID[1:]
}

func renderListScopeCardTTY(ow *terminal.OutputWriter, scopeDisplay string, deps []string) {
	ow.PrintNewLines(1)

	badge := terminal.Green + terminal.Bold + "OK" + terminal.Reset

	if len(deps) == 0 {
		badge = terminal.Yellow + terminal.Bold + "EMPTY" + terminal.Reset
	}

	header := fmt.Sprintf("  %s%s%s  %s",
		terminal.Bold, scopeDisplay, terminal.Reset,
		badge,
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
		_, _ = fmt.Fprintf(ow, "%s%s%s %s%s\n",
			terminal.Red, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CrossMark,
			"No dependencies found.", terminal.Reset,
		)

		return
	}

	for _, dep := range deps {
		_, _ = fmt.Fprintf(ow, "%s%s%s %s%s\n",
			terminal.Green, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CheckMark,
			dep, terminal.Reset,
		)
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
