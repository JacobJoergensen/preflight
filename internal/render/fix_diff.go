package render

import (
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const (
	maxDiffEntriesPerFile     = 20
	lockDiffSubheaderRuleSize = 25
)

func renderFixDiffs(ow *terminal.OutputWriter, report result.FixReport) {
	if !report.Diff || report.DryRun {
		return
	}

	if report.SkipBackup {
		ow.PrintNewLines(1)
		ow.Println(terminal.Bold + "Lock file changes" + terminal.Reset)
		ow.Println(terminal.Dim + "  " + terminal.WarningSign + " Lock file diff unavailable (backup skipped)." + terminal.Reset)
		return
	}

	if len(report.LockDiffs) == 0 {
		return
	}

	ow.PrintNewLines(1)
	ow.Println(terminal.Bold + "Lock file changes" + terminal.Reset)

	for _, diff := range report.LockDiffs {
		renderFileDiff(ow, diff)
	}
}

func renderFileDiff(ow *terminal.OutputWriter, diff lockdiff.FileDiff) {
	added, removed, upgraded, downgraded := diff.Counts()

	ow.PrintNewLines(1)
	ow.Printf("  %s%s%s %s· %s%s\n",
		terminal.Bold, diff.File, terminal.Reset,
		terminal.Dim, diff.Ecosystem, terminal.Reset,
	)
	ow.Println("  " + terminal.Gray + strings.Repeat("─", lockDiffSubheaderRuleSize) + terminal.Reset)

	shown := 0

	for _, change := range diff.Changes {
		if shown >= maxDiffEntriesPerFile {
			remaining := len(diff.Changes) - shown
			ow.Printf("%s%s… %d more change%s hidden%s\n",
				terminal.Dim, strings.Repeat(" ", ttyProjectBodySpaces),
				remaining, pluralSuffix(remaining), terminal.Reset,
			)

			break
		}

		ow.Println(formatChangeLine(change))
		shown++
	}

	ow.PrintNewLines(1)
	ow.Println(terminal.Dim + "  " + summarizeDiffCounts(added, removed, upgraded, downgraded, diff.MajorUpgrades()) + terminal.Reset)
}

func formatChangeLine(change lockdiff.PackageChange) string {
	marker, markerColor := changeMarker(change.Kind)
	indent := strings.Repeat(" ", ttyProjectBodySpaces)

	switch change.Kind {
	case lockdiff.ChangeAdded:
		return fmt.Sprintf("%s%s%s %s %s%s%s",
			markerColor, indent, marker, change.Name,
			terminal.Green, change.ToVer, terminal.Reset,
		)
	case lockdiff.ChangeRemoved:
		return fmt.Sprintf("%s%s%s %s %s(was %s)%s",
			markerColor, indent, marker, change.Name,
			terminal.Dim, change.FromVer, terminal.Reset,
		)
	case lockdiff.ChangeUpgraded, lockdiff.ChangeDowngraded:
		return fmt.Sprintf("%s%s%s %s %s%s%s → %s%s%s %s(%s)%s",
			markerColor, indent, marker, change.Name,
			terminal.Dim, change.FromVer, terminal.Reset,
			levelColor(change.Level), change.ToVer, terminal.Reset,
			terminal.Dim, string(change.Level), terminal.Reset,
		)
	}

	return ""
}

func changeMarker(kind lockdiff.ChangeKind) (marker string, color string) {
	switch kind {
	case lockdiff.ChangeAdded:
		return "+", terminal.Green
	case lockdiff.ChangeRemoved:
		return "-", terminal.Red
	case lockdiff.ChangeUpgraded:
		return "↑", terminal.Cyan
	case lockdiff.ChangeDowngraded:
		return "↓", terminal.Yellow
	}

	return " ", terminal.Reset
}

func levelColor(level lockdiff.SemverLevel) string {
	switch level {
	case lockdiff.LevelMajor:
		return terminal.Yellow + terminal.Bold
	case lockdiff.LevelMinor:
		return terminal.Cyan
	case lockdiff.LevelPatch:
		return terminal.Green
	}

	return terminal.Reset
}

func summarizeDiffCounts(added, removed, upgraded, downgraded, majorUpgrades int) string {
	parts := make([]string, 0, 4)

	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", added))
	}

	if removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", removed))
	}

	if upgraded > 0 {
		entry := fmt.Sprintf("%d upgraded", upgraded)

		if majorUpgrades > 0 {
			entry += fmt.Sprintf(" (%d major)", majorUpgrades)
		}

		parts = append(parts, entry)
	}

	if downgraded > 0 {
		parts = append(parts, fmt.Sprintf("%d downgraded", downgraded))
	}

	if len(parts) == 0 {
		return "no changes"
	}

	return strings.Join(parts, ", ")
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
}
