package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type TTYFixRenderer struct {
	Out *terminal.OutputWriter
}

func (r TTYFixRenderer) Render(report result.FixReport) error {
	ow := r.Out

	if ow == nil {
		ow = terminal.NewOutputWriter()
	}

	if terminal.Quiet {
		return renderFixQuiet(ow, report)
	}

	title := terminal.Wrench + " PreFlight Fix  "

	if report.DryRun {
		title = terminal.Wrench + " PreFlight Fix (Dry Run)  "
	}

	ow.Println(terminal.Bold + terminal.Blue + "\n╭─────────────────────────────────────────╮" + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "│" + terminal.Cyan + terminal.Bold + "  " + title + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "╰─────────────────────────────────────────╯" + terminal.Reset)

	if report.DryRun {
		ow.PrintNewLines(1)
		renderFixPlan(ow, report)
		renderFixFooter(ow, report)
		return nil
	}

	ow.Println(terminal.Bold + "\nFixing dependencies..." + terminal.Reset)
	ow.PrintNewLines(1)

	renderedCount := 0

	for _, item := range report.Items {
		if shouldSkipFixItem(item) {
			continue
		}

		renderFixItemCardTTY(ow, item)
		renderedCount++
	}

	if renderedCount == 0 && len(report.Items) > 0 {
		ow.Println(terminal.Dim + "  " + terminal.CheckMark + " All dependencies already up to date." + terminal.Reset)
	}

	renderFixSkipped(ow, report.Skipped)
	renderFixDiffs(ow, report)
	renderFixFooter(ow, report)

	return nil
}

func renderFixPlan(ow *terminal.OutputWriter, report result.FixReport) {
	if len(report.Plan) == 0 {
		ow.Println(terminal.Dim + "  " + terminal.WarningSign + " No package managers to fix." + terminal.Reset)
		return
	}

	header := fmt.Sprintf("Plan — %d ecosystem%s", len(report.Plan), pluralSuffix(len(report.Plan)))
	ow.Println(terminal.Bold + header + terminal.Reset)
	ow.Println(terminal.Gray + strings.Repeat("─", checkCardRuleWidth) + terminal.Reset)

	for _, planned := range report.Plan {
		ow.PrintNewLines(1)
		ow.Printf("  %s%s%s\n", terminal.Bold, planned.DisplayName, terminal.Reset)

		if planned.Command != "" {
			ow.Printf("    %s→%s %s\n", terminal.Cyan, terminal.Reset, planned.Command)
		}

		if planned.Summary != "" {
			ow.Printf("    %s%s%s\n", terminal.Dim, planned.Summary, terminal.Reset)
		}
	}
}

func renderFixSkipped(ow *terminal.OutputWriter, skipped []result.SkippedFix) {
	if len(skipped) == 0 {
		return
	}

	ow.PrintNewLines(1)
	ow.Println(terminal.Bold + "Skipped" + terminal.Reset)

	for _, entry := range skipped {
		label := entry.DisplayName

		if label == "" {
			label = entry.ScopeID
		}

		detail := entry.Command

		if detail == "" {
			detail = entry.Reason
		}

		ow.Printf("%s  · %s%s  %s%s%s\n",
			terminal.Yellow, label, terminal.Reset,
			terminal.Dim, detail, terminal.Reset,
		)
	}
}

func renderFixItemCardTTY(ow *terminal.OutputWriter, item result.FixItem) {
	ow.PrintNewLines(1)

	badge := fixBadgeTTY(item)
	header := fmt.Sprintf("  %s%s%s  %s",
		terminal.Bold, item.ManagerName, terminal.Reset,
		badge,
	)

	ow.Println(header)

	summary := fixItemSummary(item)

	if summary != "" {
		ow.Println(terminal.Dim + "  " + summary + terminal.Reset)
	}

	ow.Println(terminal.Gray + strings.Repeat("─", checkCardRuleWidth) + terminal.Reset)

	fullCommand := buildFullCommand(item.ManagerCommand, item.Args)

	ow.Println(terminal.Dim + "  Result" + terminal.Reset)

	if item.Success {
		if fullCommand != "" {
			ow.Printf("%s%s%s Ran %s%s\n",
				terminal.Green, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CheckMark,
				fullCommand, terminal.Reset,
			)
		} else {
			ow.Printf("%s%s%s %s%s\n",
				terminal.Green, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CheckMark,
				"Already up to date", terminal.Reset,
			)
		}

		return
	}

	if item.Error != "" {
		ow.Printf("%s%s%s %s%s\n",
			terminal.Red, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CrossMark,
			item.Error, terminal.Reset,
		)
	}
}

func buildFullCommand(command string, args []string) string {
	if command == "" {
		return ""
	}

	if len(args) == 0 {
		return command
	}

	return command + " " + strings.Join(args, " ")
}

func shouldSkipFixItem(item result.FixItem) bool {
	if item.Error != "" {
		return false
	}

	if item.ManagerCommand != "" && len(item.Args) > 0 {
		return false
	}

	return item.Success
}

func fixBadgeTTY(item result.FixItem) string {
	if item.Success {
		return terminal.Green + terminal.Bold + "OK" + terminal.Reset
	}

	return terminal.Red + terminal.Bold + "FAIL" + terminal.Reset
}

func fixItemSummary(item result.FixItem) string {
	if item.Success {
		return "Dependencies installed successfully"
	}

	return "Installation failed"
}

func renderFixFooter(ow *terminal.OutputWriter, report result.FixReport) {
	statusIcon, statusColor, statusText := fixStatusFromReport(report)

	endedAt := report.EndedAt

	if endedAt.IsZero() {
		endedAt = time.Now()
	}

	ow.Println(terminal.Bold + terminal.Blue + "\n╭────────────────────────────────────────────────────────────────╮" + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "│ " + statusColor + statusIcon + " Status: " + statusText + terminal.Reset)

	if report.BackupDir != "" {
		ow.Println(terminal.Bold + terminal.Blue + "│ " + terminal.Dim + terminal.Box + " Backup: " + report.BackupDir + terminal.Reset)
	}

	ow.Println(terminal.Bold + terminal.Blue + "│ " + terminal.Dim + terminal.Clock + " Ended: " + endedAt.Format("02-01-2006 15:04:05") + terminal.Reset)
	ow.Println(terminal.Bold + terminal.Blue + "╰────────────────────────────────────────────────────────────────╯" + terminal.Reset)
}

func fixStatusFromReport(report result.FixReport) (icon string, color string, text string) {
	if report.Canceled {
		return terminal.WarningSign, terminal.Yellow, "Fix canceled."
	}

	if report.Aborted {
		return terminal.WarningSign, terminal.Yellow, "Fix aborted — no changes applied."
	}

	if report.DryRun {
		if len(report.Plan) == 0 {
			return terminal.WarningSign, terminal.Yellow, "No package managers to fix."
		}

		return terminal.CheckMark, terminal.Cyan, "Dry run completed, no changes made."
	}

	if len(report.Items) == 0 && len(report.Skipped) > 0 {
		return terminal.WarningSign, terminal.Yellow, "Nothing applied — all ecosystems skipped."
	}

	if len(report.Items) == 0 {
		return terminal.WarningSign, terminal.Yellow, "No package managers to fix."
	}

	var failCount int

	for _, item := range report.Items {
		if !item.Success {
			failCount++
		}
	}

	if failCount > 0 {
		return terminal.CrossMark, terminal.Red, fmt.Sprintf("Fix completed with %s.", fixFailPhrase(failCount))
	}

	return terminal.CheckMark, terminal.Green, "All dependencies fixed successfully!"
}

func fixFailPhrase(count int) string {
	if count == 1 {
		return "1 failure"
	}

	return fmt.Sprintf("%d failures", count)
}

func renderFixQuiet(ow *terminal.OutputWriter, report result.FixReport) error {
	for _, item := range report.Items {
		if item.Success {
			continue
		}

		if item.Error != "" {
			ow.Println(item.ManagerName + ": " + item.Error)
		}
	}

	return nil
}
