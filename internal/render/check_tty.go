package render

import (
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const (
	checkCardRuleWidth   = 64
	ttyProjectBodySpaces = 4
)

type TTYCheckRenderer struct {
	Out *terminal.OutputWriter
}

func (r TTYCheckRenderer) Render(report result.CheckReport) error {
	ow := r.Out

	if ow == nil {
		ow = terminal.NewOutputWriter()
	}

	if terminal.Quiet {
		return renderCheckQuiet(ow, report)
	}

	ow.PrintNewLines(1)

	renderCheckItemsGroupedByProject(ow, report)

	icon, color, text := statusFromReport(report)
	renderStatusFooter(ow, footerStatus{Icon: icon, Color: color, Text: text}, []footerMetadataLine{endedFooterLine(report.EndedAt)})

	return nil
}

func renderCheckItemsGroupedByProject(ow *terminal.OutputWriter, report result.CheckReport) {
	renderByProject(ow, report.Projects, report.Items,
		func(p result.Project) string { return p.RelativePath },
		func(i result.CheckItem) string { return i.Project },
		renderProjectHeader,
		func(ow *terminal.OutputWriter, item result.CheckItem) {
			card := BuildHealthCard(item)
			renderHealthCardTTY(ow, card, item.Outdated)
		},
	)
}

func renderProjectHeader(ow *terminal.OutputWriter, project result.Project) {
	line := "  " + terminal.Bold + terminal.Cyan + project.RelativePath + terminal.Reset

	if project.Name != "" {
		line += "  " + terminal.Dim + project.Name + terminal.Reset
	}

	ow.Println(line)
}

func renderHealthCardTTY(ow *terminal.OutputWriter, card HealthCard, outdated []ecosystem.OutdatedPackage) {
	ow.PrintNewLines(1)

	badge := healthBadgeTTY(card.Status)
	elapsed := ""

	if card.ElapsedMillis > 0 {
		elapsed = terminal.Dim + fmt.Sprintf(" %dms", card.ElapsedMillis) + terminal.Reset
	}

	outdatedIndicator := ""

	if len(outdated) > 0 {
		outdatedIndicator = "  " + terminal.Yellow + terminal.Lightning + " " + fmt.Sprintf("%d outdated", len(outdated)) + terminal.Reset
	}

	header := fmt.Sprintf("  %s%s%s  %s%s%s",
		terminal.Bold, card.ScopeDisplay, terminal.Reset,
		badge, elapsed, outdatedIndicator,
	)

	ow.Println(header)

	if card.Summary != "" {
		ow.Println(terminal.Dim + "  " + card.Summary + terminal.Reset)
	}

	ow.Println(terminal.Gray + strings.Repeat("─", checkCardRuleWidth) + terminal.Reset)

	printSection := func(title string) {
		ow.Println(terminal.Dim + "  " + title + terminal.Reset)
	}

	if len(card.Toolchain) > 0 {
		printSection("Toolchain")
		printToolchainLinesTTY(ow, card.Toolchain)
	}

	if len(card.Signals) > 0 {
		printSection("Project")
		printIndentedLines(ow, card.Signals, terminal.Gray+strings.Repeat(" ", ttyProjectBodySpaces)+terminal.Reset)
	}

	if len(card.FlatWarnings) > 0 || len(card.FlatErrors) > 0 {
		printSection("Issues")

		printMessages(ow, card.FlatWarnings, terminal.Yellow, terminal.WarningSign)
		printMessages(ow, card.FlatErrors, terminal.Red, terminal.CrossMark)
	}

	outdatedByName := buildOutdatedMap(outdated)

	hasProdDeps := len(card.DepSuccess) > 0 || len(card.DepWarnings) > 0 || len(card.DepErrors) > 0
	hasDevDeps := len(card.DepDevSuccess) > 0 || len(card.DepDevWarnings) > 0 || len(card.DepDevErrors) > 0
	hasOptionalDeps := len(card.DepOptionalSuccess) > 0 || len(card.DepOptionalWarnings) > 0 || len(card.DepOptionalInfo) > 0

	if hasProdDeps || hasDevDeps || hasOptionalDeps {
		if hasProdDeps {
			printSection("Dependencies")
			printDepSuccesses(ow, card.DepSuccess, outdatedByName)
			printMessagesUniformCapped(ow, card.DepWarnings, terminal.Yellow, terminal.WarningSign, "dependency warnings")
			printMessagesUniformCapped(ow, card.DepErrors, terminal.Red, terminal.CrossMark, "dependency errors")
		}

		if hasDevDeps {
			printSection("Dev dependencies")
			printDepSuccesses(ow, card.DepDevSuccess, outdatedByName)
			printMessagesUniformCapped(ow, card.DepDevWarnings, terminal.Yellow, terminal.WarningSign, "dev dependency warnings")
			printMessagesUniformCapped(ow, card.DepDevErrors, terminal.Red, terminal.CrossMark, "dev dependency errors")
		}

		if hasOptionalDeps {
			printSection("Optional dependencies")
			printDepSuccesses(ow, card.DepOptionalSuccess, outdatedByName)
			printMessagesUniformCapped(ow, card.DepOptionalWarnings, terminal.Yellow, terminal.WarningSign, "optional dependency warnings")
			printOptionalInfoLines(ow, card.DepOptionalInfo)
		}
	}

	if card.PrimaryNextStep != "" {
		printSection("Next step")

		ow.Println(terminal.Cyan + strings.Repeat(" ", ttyProjectBodySpaces) + "› " + card.PrimaryNextStep + terminal.Reset)
	}
}

func printIndentedLines(ow *terminal.OutputWriter, lines []string, prefix string) {
	for _, line := range lines {
		ow.Println(prefix + line)
	}
}

func healthBadgeTTY(status HealthStatus) string {
	switch status {
	case HealthOK:
		return terminal.Green + terminal.Bold + "OK" + terminal.Reset
	case HealthWarn:
		return terminal.Yellow + terminal.Bold + "WARN" + terminal.Reset
	case HealthFail:
		return terminal.Red + terminal.Bold + "FAIL" + terminal.Reset
	case HealthSkip:
		return terminal.Gray + terminal.Bold + "SKIP" + terminal.Reset
	default:
		return terminal.Gray + string(status) + terminal.Reset
	}
}

func renderCheckQuiet(ow *terminal.OutputWriter, report result.CheckReport) error {
	for _, item := range report.Items {
		outdated := item.Outdated

		if len(item.Errors()) == 0 && len(item.Warnings()) == 0 && len(outdated) == 0 {
			continue
		}

		card := BuildHealthCard(item)
		header := card.ScopeDisplay

		if card.Status != HealthOK {
			header += "  " + strings.ToUpper(string(card.Status))
		}

		if len(outdated) > 0 {
			header += "  " + terminal.Yellow + terminal.Lightning + " " + fmt.Sprintf("%d outdated", len(outdated)) + terminal.Reset
		}

		ow.Println(header)

		if card.Summary != "" {
			ow.Println(terminal.Dim + "  " + card.Summary + terminal.Reset)
		}

		if card.PrimaryNextStep != "" {
			ow.Println(terminal.Cyan + "  › " + card.PrimaryNextStep + terminal.Reset)
		}

		if len(card.FlatErrors) > 0 {
			printMessages(ow, card.FlatErrors, terminal.Red, terminal.CrossMark)
		}

		if len(card.FlatWarnings) > 0 {
			printMessages(ow, card.FlatWarnings, terminal.Yellow, terminal.WarningSign)
		}

		if len(card.DepErrors) > 0 {
			printMessagesUniformCapped(ow, card.DepErrors, terminal.Red, terminal.CrossMark, "dependency errors")
		}

		if len(card.DepDevErrors) > 0 {
			printMessagesUniformCapped(ow, card.DepDevErrors, terminal.Red, terminal.CrossMark, "dev dependency errors")
		}

		if len(card.DepWarnings) > 0 {
			printMessagesUniformCapped(ow, card.DepWarnings, terminal.Yellow, terminal.WarningSign, "dependency warnings")
		}

		if len(card.DepDevWarnings) > 0 {
			printMessagesUniformCapped(ow, card.DepDevWarnings, terminal.Yellow, terminal.WarningSign, "dev dependency warnings")
		}

		if len(card.DepOptionalWarnings) > 0 {
			printMessagesUniformCapped(ow, card.DepOptionalWarnings, terminal.Yellow, terminal.WarningSign, "optional dependency warnings")
		}

		printOutdatedLinesQuietTTY(ow, outdated)
	}

	return nil
}

func printOutdatedRow(ow *terminal.OutputWriter, pkg ecosystem.OutdatedPackage) {
	ow.Printf("%s%s%s %s %s%s%s → %s%s%s\n",
		terminal.Yellow, strings.Repeat(" ", ttyProjectBodySpaces), terminal.Lightning,
		pkg.Name,
		terminal.Dim, pkg.Current, terminal.Reset,
		terminal.Green, pkg.Latest, terminal.Reset,
	)
}

func printOutdatedLinesQuietTTY(ow *terminal.OutputWriter, outdated []ecosystem.OutdatedPackage) {
	for _, pkg := range outdated {
		printOutdatedRow(ow, pkg)
	}
}

func printMessages(ow *terminal.OutputWriter, messages []model.Message, color string, symbol string) {
	for _, msg := range messages {
		indent := ttyProjectBodySpaces
		if msg.Nested {
			indent = ttyProjectBodySpaces + 2
		}

		lines := strings.Split(msg.Text, "\n")
		ow.Printf("%s%s%s %s\n", color, strings.Repeat(" ", indent), symbol, lines[0])

		continuationPad := strings.Repeat(" ", indent+2)

		for _, line := range lines[1:] {
			ow.Printf("%s%s%s %s\n", color, continuationPad, terminal.Bullet, line)
		}
	}
}

func printOptionalInfoLines(ow *terminal.OutputWriter, messages []model.Message) {
	for _, msg := range messages {
		ow.Printf("%s%s%s%s\n",
			terminal.Dim, strings.Repeat(" ", ttyProjectBodySpaces),
			msg.Text, terminal.Reset,
		)
	}
}

func printMessagesUniform(ow *terminal.OutputWriter, messages []model.Message, color string, symbol string) {
	for _, msg := range messages {
		ow.Printf("%s%s%s %s\n", color, strings.Repeat(" ", ttyProjectBodySpaces), symbol, msg.Text)
	}
}

func buildOutdatedMap(packages []ecosystem.OutdatedPackage) map[string]ecosystem.OutdatedPackage {
	if len(packages) == 0 {
		return nil
	}

	m := make(map[string]ecosystem.OutdatedPackage, len(packages))

	for _, pkg := range packages {
		m[strings.ToLower(pkg.Name)] = pkg
	}

	return m
}

func printDepSuccesses(ow *terminal.OutputWriter, deps []model.Message, outdated map[string]ecosystem.OutdatedPackage) {
	if len(deps) == 0 {
		return
	}

	if terminal.Verbose || len(deps) <= maxDepRowsPerSection {
		printDepLinesWithOutdated(ow, deps, outdated)
		return
	}

	noun := depNoun(deps[0].Text, len(deps))

	ow.Printf("%s%s%s %d %s installed%s  %s(--verbose for all)%s\n",
		terminal.Green, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CheckMark, len(deps), noun, terminal.Reset,
		terminal.Dim, terminal.Reset,
	)

	for _, msg := range deps {
		name := extractDepName(msg.Text)

		if pkg, isOutdated := outdated[strings.ToLower(name)]; isOutdated {
			printOutdatedRow(ow, pkg)
		}
	}
}

func depNoun(sample string, count int) string {
	noun := "dependency"

	if fields := strings.Fields(stripANSI(sample)); len(fields) >= 2 && fields[0] == "Installed" {
		noun = fields[1]
	}

	if count == 1 {
		return noun
	}

	if strings.HasSuffix(noun, "y") {
		return noun[:len(noun)-1] + "ies"
	}

	return noun + "s"
}

func printDepLinesWithOutdated(ow *terminal.OutputWriter, deps []model.Message, outdated map[string]ecosystem.OutdatedPackage) {
	for _, msg := range deps {
		name := extractDepName(msg.Text)
		pkg, isOutdated := outdated[strings.ToLower(name)]

		if isOutdated {
			printOutdatedRow(ow, pkg)
		} else {
			ow.Printf("%s%s%s %s\n",
				terminal.Green, strings.Repeat(" ", ttyProjectBodySpaces), terminal.CheckMark, msg.Text,
			)
		}
	}
}

func extractDepName(text string) string {
	text = strings.TrimSpace(text)

	prefixes := []string{
		"Installed dependency ",
		"Installed package ",
		"Installed module ",
		"Installed gem ",
		"Installed ",
	}

	for _, prefix := range prefixes {
		if dep, ok := strings.CutPrefix(text, prefix); ok {
			dep = stripANSI(dep)

			if i := strings.Index(dep, " ("); i > 0 {
				return dep[:i]
			}

			return dep
		}
	}

	return text
}

func stripANSI(s string) string {
	var b strings.Builder

	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			for i < len(s) && s[i] != 'm' {
				i++
			}

			continue
		}

		b.WriteByte(s[i])
	}

	return b.String()
}

func statusFromReport(report result.CheckReport) (icon string, color string, text string) {
	if report.Canceled {
		return terminal.WarningSign, terminal.Yellow, "Checks canceled."
	}

	if len(report.Projects) > 0 {
		return monorepoStatusFromReport(report)
	}

	var totalErrors, totalWarnings int

	for _, item := range report.Items {
		totalErrors += len(item.Errors())
		totalWarnings += len(item.Warnings())
	}

	if totalErrors > 0 {
		return terminal.CrossMark, terminal.Red, "Errors found."
	}

	if totalWarnings > 0 {
		return terminal.WarningSign, terminal.Yellow, "Warnings found."
	}

	return terminal.CheckMark, terminal.Green, "All checks passed."
}

func monorepoStatusFromReport(report result.CheckReport) (icon string, color string, text string) {
	errorProjects := countProjects(report.Items, func(i result.CheckItem) (string, bool) { return i.Project, len(i.Errors()) > 0 })
	warningProjects := countProjects(report.Items, func(i result.CheckItem) (string, bool) { return i.Project, len(i.Warnings()) > 0 })

	totalProjects := len(report.Projects)

	if errorProjects > 0 {
		return terminal.CrossMark, terminal.Red, projectStatusLine(errorProjects, totalProjects, "reported errors")
	}

	if warningProjects > 0 {
		return terminal.WarningSign, terminal.Yellow, projectStatusLine(warningProjects, totalProjects, "reported warnings")
	}

	return terminal.CheckMark, terminal.Green, fmt.Sprintf("%d project%s checked, all healthy", totalProjects, pluralSuffix(totalProjects))
}
