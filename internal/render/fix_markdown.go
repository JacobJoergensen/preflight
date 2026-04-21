package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
)

type MarkdownFixRenderer struct {
	Out io.Writer
}

func (r MarkdownFixRenderer) Render(report result.FixReport) error {
	var doc strings.Builder

	doc.WriteString("## 🔧 PreFlight Fix\n\n")

	if report.DryRun {
		doc.WriteString("**Mode:** Dry run (no changes made)\n\n")
	}

	symbol, text := markdownFixStatus(report)
	fmt.Fprintf(&doc, "**Status:** %s %s\n\n", symbol, text)

	if report.DryRun {
		writeMarkdownFixPlan(&doc, report.Plan)
	} else {
		writeMarkdownFixResults(&doc, report.Items)
	}

	if len(report.Skipped) > 0 {
		writeMarkdownFixSkipped(&doc, report.Skipped)
	}

	if len(report.LockDiffs) > 0 {
		writeMarkdownFixLockDiffs(&doc, report.LockDiffs)
	}

	writeMarkdownFixFailureDetails(&doc, report.Items)

	doc.WriteString("\n---\n\n")

	_, err := r.Out.Write([]byte(doc.String()))
	return err
}

func markdownFixStatus(report result.FixReport) (symbol, text string) {
	switch {
	case report.Canceled:
		return "⏸", "Fix canceled"
	case report.Aborted:
		return "⏸", "Fix aborted — no changes applied"
	case report.DryRun && len(report.Plan) == 0:
		return "⚠", "No package managers to fix"
	case report.DryRun:
		return "✓", "Dry run completed, no changes made"
	case len(report.Items) == 0 && len(report.Skipped) > 0:
		return "⚠", "Nothing applied — all ecosystems skipped"
	case len(report.Items) == 0:
		return "⚠", "No package managers to fix"
	}

	var failures int

	for _, item := range report.Items {
		if !item.Success {
			failures++
		}
	}

	if failures > 0 {
		return "✗", fmt.Sprintf("Fix completed with %d failure%s", failures, pluralSuffix(failures))
	}

	return "✓", "All dependencies fixed successfully"
}

func writeMarkdownFixPlan(doc *strings.Builder, plan []result.PlannedFix) {
	if len(plan) == 0 {
		return
	}

	doc.WriteString("### Plan\n\n")
	doc.WriteString("| Ecosystem | Command | Summary |\n")
	doc.WriteString("|-----------|---------|---------|\n")

	for _, planned := range plan {
		command := "—"

		if planned.Command != "" {
			command = "`" + planned.Command + "`"
		}

		summary := escapeMarkdownCell(planned.Summary)

		if summary == "" {
			summary = "—"
		}

		fmt.Fprintf(doc, "| %s | %s | %s |\n",
			escapeMarkdownCell(planned.DisplayName),
			command,
			summary,
		)
	}

	doc.WriteString("\n")
}

func writeMarkdownFixResults(doc *strings.Builder, items []result.FixItem) {
	if len(items) == 0 {
		return
	}

	doc.WriteString("### Results\n\n")
	doc.WriteString("| Ecosystem | Status | Command | Elapsed |\n")
	doc.WriteString("|-----------|--------|---------|---------|\n")

	for _, item := range items {
		symbol := "✓"

		if !item.Success {
			symbol = "✗"
		}

		command := buildFullCommand(item.ManagerCommand, item.Args)

		if command == "" {
			command = "_already up to date_"
		} else {
			command = "`" + command + "`"
		}

		elapsed := formatFixElapsed(item.EndedAt.Sub(item.StartedAt))

		if elapsed == "" {
			elapsed = "—"
		}

		fmt.Fprintf(doc, "| %s | %s | %s | %s |\n",
			escapeMarkdownCell(item.ManagerName),
			symbol,
			command,
			elapsed,
		)
	}

	doc.WriteString("\n")
}

func writeMarkdownFixSkipped(doc *strings.Builder, skipped []result.SkippedFix) {
	doc.WriteString("### Skipped\n\n")

	for _, entry := range skipped {
		label := entry.DisplayName

		if label == "" {
			label = entry.ScopeID
		}

		fmt.Fprintf(doc, "- **%s** — %s\n",
			escapeMarkdownCell(label),
			escapeMarkdownCell(entry.Reason),
		)
	}

	doc.WriteString("\n")
}

func writeMarkdownFixLockDiffs(doc *strings.Builder, diffs []lockdiff.FileDiff) {
	doc.WriteString("### Lock file changes\n\n")

	for _, diff := range diffs {
		fmt.Fprintf(doc, "**%s** (%s)\n\n", diff.File, diff.Ecosystem)

		for _, change := range diff.Changes {
			switch change.Kind {
			case lockdiff.ChangeAdded:
				fmt.Fprintf(doc, "- ➕ `%s` `%s`\n", change.Name, change.ToVer)
			case lockdiff.ChangeRemoved:
				fmt.Fprintf(doc, "- ➖ `%s` (was `%s`)\n", change.Name, change.FromVer)
			case lockdiff.ChangeUpgraded:
				fmt.Fprintf(doc, "- ⬆️ `%s` `%s` → `%s` (%s)\n",
					change.Name, change.FromVer, change.ToVer, change.Level,
				)
			case lockdiff.ChangeDowngraded:
				fmt.Fprintf(doc, "- ⬇️ `%s` `%s` → `%s` (%s)\n",
					change.Name, change.FromVer, change.ToVer, change.Level,
				)
			}
		}

		doc.WriteString("\n")
	}
}

func writeMarkdownFixFailureDetails(doc *strings.Builder, items []result.FixItem) {
	for _, item := range items {
		if item.Success {
			continue
		}

		details := strings.TrimSpace(item.Output)

		if details == "" {
			details = strings.TrimSpace(item.Error)
		}

		if details == "" {
			continue
		}

		fmt.Fprintf(doc, "<details><summary>%s output</summary>\n\n", escapeMarkdownCell(item.ManagerName))
		doc.WriteString("```\n")
		doc.WriteString(details)
		doc.WriteString("\n```\n\n</details>\n\n")
	}
}
