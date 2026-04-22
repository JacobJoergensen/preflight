package render

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const (
	fixItemNoCommandLabel   = "already up to date"
	fixResultsRuleWidth     = 45
	fixFailureOutputIndent  = 6
	fixFailureOutputMaxRows = 40
	fixProgressSpinnerLabel = "installing…"
)

type TTYFixRenderer struct {
	Out       *terminal.OutputWriter
	SkipItems bool
}

func (r TTYFixRenderer) Render(report result.FixReport) error {
	ow := r.Out

	if ow == nil {
		ow = terminal.NewOutputWriter()
	}

	if terminal.Quiet {
		return renderFixQuiet(ow, report)
	}

	if !r.SkipItems {
		ow.PrintNewLines(1)
	}

	if report.DryRun {
		renderFixPlan(ow, report)
		renderFixFooter(ow, report)
		return nil
	}

	if !r.SkipItems {
		renderFixItemLines(ow, report)
	}

	renderFixSkipped(ow, report.Skipped)
	renderFixDiffs(ow, report)
	renderFixFooter(ow, report)

	return nil
}

func renderFixItemLines(ow *terminal.OutputWriter, report result.FixReport) {
	if len(report.Items) == 0 {
		return
	}

	ow.Println(terminal.Bold + "Results" + terminal.Reset)
	ow.Println(terminal.Gray + strings.Repeat("─", fixResultsRuleWidth) + terminal.Reset)

	nameWidth, commandWidth := fixItemColumnWidths(report.Items)

	renderByProject(ow, report.Projects, report.Items,
		func(p result.FixProject) string { return p.RelativePath },
		func(i result.FixItem) string { return i.Project },
		func(ow *terminal.OutputWriter, p result.FixProject) {
			ow.Println("  " + terminal.Bold + terminal.Cyan + p.RelativePath + terminal.Reset)
		},
		func(ow *terminal.OutputWriter, item result.FixItem) {
			renderFixItemLine(ow, item, nameWidth, commandWidth)
		},
	)
}

func renderFixItemLine(ow *terminal.OutputWriter, item result.FixItem, nameWidth, commandWidth int) {
	icon, iconColor := fixItemIcon(item)
	command := buildFullCommand(item.ManagerCommand, item.Args)

	if command == "" {
		command = fixItemNoCommandLabel
	}

	elapsed := formatFixElapsed(item.EndedAt.Sub(item.StartedAt))

	ow.Printf("  %s%s%s  %s%s%s  %s%s%s  %s%s%s\n",
		iconColor, icon, terminal.Reset,
		terminal.Bold, padRight(item.ManagerName, nameWidth), terminal.Reset,
		terminal.Dim, padRight(command, commandWidth), terminal.Reset,
		terminal.Dim, elapsed, terminal.Reset,
	)

	if item.Success {
		return
	}

	if strings.TrimSpace(item.Output) != "" {
		renderFixItemCapturedOutput(ow, item.Output)
		return
	}

	if item.Error != "" {
		ow.Printf("%s%s%s%s\n",
			terminal.Red, strings.Repeat(" ", ttyProjectBodySpaces),
			item.Error, terminal.Reset,
		)
	}
}

func renderFixItemCapturedOutput(ow *terminal.OutputWriter, output string) {
	lines := capturedOutputLines(output)

	if len(lines) == 0 {
		return
	}

	ow.PrintNewLines(1)

	indent := strings.Repeat(" ", fixFailureOutputIndent)

	for _, line := range lines {
		ow.Printf("%s%s%s%s\n", terminal.Red+terminal.Dim, indent, line, terminal.Reset)
	}

	ow.PrintNewLines(1)
}

func capturedOutputLines(output string) []string {
	trimmed := strings.TrimRight(output, "\n")

	if trimmed == "" {
		return nil
	}

	lines := strings.Split(trimmed, "\n")

	if len(lines) <= fixFailureOutputMaxRows {
		return lines
	}

	skipped := len(lines) - fixFailureOutputMaxRows
	tail := lines[skipped:]
	truncated := make([]string, 0, len(tail)+1)
	truncated = append(truncated, fmt.Sprintf("… %d earlier line%s hidden …", skipped, pluralSuffix(skipped)))
	truncated = append(truncated, tail...)

	return truncated
}

func fixItemColumnWidths(items []result.FixItem) (nameWidth, commandWidth int) {
	for _, item := range items {
		if n := len(item.ManagerName); n > nameWidth {
			nameWidth = n
		}

		command := buildFullCommand(item.ManagerCommand, item.Args)

		if command == "" {
			command = fixItemNoCommandLabel
		}

		if n := len(command); n > commandWidth {
			commandWidth = n
		}
	}

	return nameWidth, commandWidth
}

func fixItemIcon(item result.FixItem) (icon, color string) {
	if item.Success {
		return terminal.CheckMark, terminal.Green
	}

	return terminal.CrossMark, terminal.Red
}

func formatFixElapsed(d time.Duration) string {
	if d <= 0 {
		return ""
	}

	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}

	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}

	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}

	return s + strings.Repeat(" ", width-len(s))
}

func renderFixPlan(ow *terminal.OutputWriter, report result.FixReport) {
	if len(report.Plan) == 0 {
		ow.Println(terminal.Dim + "  " + terminal.WarningSign + " No package managers to fix." + terminal.Reset)
		return
	}

	header := fmt.Sprintf("Plan — %d ecosystem%s", len(report.Plan), pluralSuffix(len(report.Plan)))
	ow.Println(terminal.Bold + header + terminal.Reset)
	ow.Println(terminal.Gray + strings.Repeat("─", checkCardRuleWidth) + terminal.Reset)

	if len(report.Projects) == 0 {
		for _, planned := range report.Plan {
			renderPlannedFixBlock(ow, planned)
		}

		return
	}

	planByProject := make(map[string][]result.PlannedFix, len(report.Projects))

	for _, planned := range report.Plan {
		planByProject[planned.Project] = append(planByProject[planned.Project], planned)
	}

	for _, project := range report.Projects {
		entries := planByProject[project.RelativePath]

		if len(entries) == 0 {
			continue
		}

		ow.PrintNewLines(1)
		ow.Println("  " + terminal.Bold + terminal.Cyan + project.RelativePath + terminal.Reset)

		for _, planned := range entries {
			renderPlannedFixBlock(ow, planned)
		}
	}
}

func renderPlannedFixBlock(ow *terminal.OutputWriter, planned result.PlannedFix) {
	ow.PrintNewLines(1)
	ow.Printf("  %s%s%s\n", terminal.Bold, planned.DisplayName, terminal.Reset)

	if planned.Command != "" {
		ow.Printf("    %s→%s %s\n", terminal.Cyan, terminal.Reset, planned.Command)
	}

	if planned.Summary != "" {
		ow.Printf("    %s%s%s\n", terminal.Dim, planned.Summary, terminal.Reset)
	}
}

func renderFixSkipped(ow *terminal.OutputWriter, skipped []result.SkippedFix) {
	if len(skipped) == 0 {
		return
	}

	ow.PrintNewLines(1)
	ow.Println(terminal.Bold + "Skipped" + terminal.Reset)

	for _, entry := range skipped {
		renderSkippedFixLine(ow, entry)
	}
}

func renderSkippedFixLine(ow *terminal.OutputWriter, entry result.SkippedFix) {
	label := entry.DisplayName

	if label == "" {
		label = entry.ScopeID
	}

	if entry.Project != "" && label != "" {
		label = entry.Project + " · " + label
	} else if entry.Project != "" {
		label = entry.Project
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

func buildFullCommand(command string, args []string) string {
	if command == "" {
		return ""
	}

	if len(args) == 0 {
		return command
	}

	return command + " " + strings.Join(args, " ")
}

func renderFixFooter(ow *terminal.OutputWriter, report result.FixReport) {
	icon, color, text := fixStatusFromReport(report)
	renderStatusFooter(ow, footerStatus{Icon: icon, Color: color, Text: text}, fixFooterMetadata(report))
}

func fixFooterMetadata(report result.FixReport) []footerMetadataLine {
	lines := make([]footerMetadataLine, 0, 2)

	if report.BackupDir != "" {
		lines = append(lines, footerMetadataLine{
			Label: "Backup",
			Value: relativeBackupPath(report.BackupDir),
		})
	}

	if len(report.BackupDirs) > 0 {
		lines = append(lines, footerMetadataLine{
			Label: "Backups",
			Value: fmt.Sprintf("%d project%s · .preflight/backups/<timestamp>", len(report.BackupDirs), pluralSuffix(len(report.BackupDirs))),
		})
	}

	lines = append(lines, endedFooterLine(report.EndedAt))

	return lines
}

func relativeBackupPath(backupDir string) string {
	idx := strings.Index(backupDir, ".preflight")

	if idx == -1 {
		return backupDir
	}

	return filepath.ToSlash(backupDir[idx:])
}

func fixStatusFromReport(report result.FixReport) (icon string, color string, text string) {
	if report.Canceled {
		return terminal.WarningSign, terminal.Yellow, "Fix canceled"
	}

	if report.Aborted {
		return terminal.WarningSign, terminal.Yellow, "Fix aborted — no changes applied"
	}

	if report.DryRun {
		if len(report.Plan) == 0 {
			return terminal.WarningSign, terminal.Yellow, "No package managers to fix"
		}

		return terminal.CheckMark, terminal.Cyan, "Dry run completed, no changes made"
	}

	if len(report.Projects) > 0 {
		return monorepoFixStatusFromReport(report)
	}

	if len(report.Items) == 0 && len(report.Skipped) > 0 {
		return terminal.WarningSign, terminal.Yellow, "Nothing applied — all ecosystems skipped"
	}

	if len(report.Items) == 0 {
		return terminal.WarningSign, terminal.Yellow, "No package managers to fix"
	}

	var failCount int

	for _, item := range report.Items {
		if !item.Success {
			failCount++
		}
	}

	if failCount > 0 {
		return terminal.CrossMark, terminal.Red, "Fix completed with " + fixFailPhrase(failCount)
	}

	return terminal.CheckMark, terminal.Green, "All dependencies fixed successfully"
}

func monorepoFixStatusFromReport(report result.FixReport) (icon string, color string, text string) {
	if len(report.Items) == 0 && len(report.Skipped) > 0 {
		return terminal.WarningSign, terminal.Yellow, "Nothing applied — all ecosystems skipped"
	}

	if len(report.Items) == 0 {
		return terminal.WarningSign, terminal.Yellow, "No package managers to fix"
	}

	projectsWithFailures := make(map[string]struct{})

	for _, item := range report.Items {
		if !item.Success {
			projectsWithFailures[item.Project] = struct{}{}
		}
	}

	totalProjects := len(report.Projects)

	if len(projectsWithFailures) > 0 {
		return terminal.CrossMark, terminal.Red, fmt.Sprintf("%d of %d project%s reported failures", len(projectsWithFailures), totalProjects, pluralSuffix(totalProjects))
	}

	return terminal.CheckMark, terminal.Green, fmt.Sprintf("All %d project%s fixed successfully", totalProjects, pluralSuffix(totalProjects))
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

		summary := quietFailureSummary(item)

		if summary == "" {
			continue
		}

		label := item.ManagerName

		if item.Project != "" {
			label = item.Project + " · " + label
		}

		ow.Println(label + ": " + summary)
	}

	return nil
}

func quietFailureSummary(item result.FixItem) string {
	trimmed := strings.TrimSpace(item.Output)

	if trimmed != "" {
		lines := strings.Split(trimmed, "\n")

		for i := len(lines) - 1; i >= 0; i-- {
			if line := strings.TrimSpace(lines[i]); line != "" {
				return line
			}
		}
	}

	return item.Error
}

type TTYFixProgress struct {
	out          *terminal.OutputWriter
	nameWidth    int
	commandWidth int

	writeMu            sync.Mutex
	headerOnce         sync.Once
	spinner            *Spinner
	currentStart       time.Time
	currentName        string
	lastPrintedProject string
	haveRenderedFirst  bool
}

func NewTTYFixProgress(writer io.Writer) *TTYFixProgress {
	return &TTYFixProgress{
		out:     terminal.NewOutputWriterFor(writer),
		spinner: NewSpinner(),
	}
}

func (p *TTYFixProgress) Plan(candidates []engine.FixCandidate) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	for _, candidate := range candidates {
		if n := len(candidate.DisplayName); n > p.nameWidth {
			p.nameWidth = n
		}

		if n := len(fixProgressCommandLabel(candidate)); n > p.commandWidth {
			p.commandWidth = n
		}
	}
}

func (p *TTYFixProgress) Start(candidate engine.FixCandidate) {
	p.headerOnce.Do(p.writeResultsHeader)

	p.writeProjectHeaderIfChanged(candidate.Project)

	p.writeMu.Lock()
	p.currentStart = time.Now()
	p.currentName = candidate.DisplayName
	p.writeMu.Unlock()

	p.paintSpinnerLine(p.spinner.Frame(0))

	p.spinner.Start(func(frame string) {
		p.paintSpinnerLine(frame)
	})
}

func (p *TTYFixProgress) Finish(item result.FixItem) {
	p.spinner.Stop()

	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	p.writeFinalLineLocked(item)

	if !item.Success {
		p.writeFailureOutputLocked(item)
	}
}

func (p *TTYFixProgress) writeProjectHeaderIfChanged(project string) {
	if project == "" {
		return
	}

	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	if project == p.lastPrintedProject {
		return
	}

	if p.haveRenderedFirst {
		p.out.PrintNewLines(1)
	}

	p.out.Println("  " + terminal.Bold + terminal.Cyan + project + terminal.Reset)
	p.lastPrintedProject = project
	p.haveRenderedFirst = true
}

func (p *TTYFixProgress) paintSpinnerLine(frame string) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	elapsed := time.Since(p.currentStart)

	p.out.Printf("%s  %s%s%s  %s%s%s  %s%s%s  %s%s%s",
		ansiClearLine,
		terminal.Cyan, frame, terminal.Reset,
		terminal.Bold, padRight(p.currentName, p.nameWidth), terminal.Reset,
		terminal.Dim, padRight(fixProgressSpinnerLabel, p.commandWidth), terminal.Reset,
		terminal.Dim, formatFixElapsed(elapsed), terminal.Reset,
	)
}

func (p *TTYFixProgress) writeFinalLineLocked(item result.FixItem) {
	icon, color := fixItemIcon(item)
	command := buildFullCommand(item.ManagerCommand, item.Args)

	if command == "" {
		command = fixItemNoCommandLabel
	}

	p.out.Printf("%s  %s%s%s  %s%s%s  %s%s%s  %s%s%s\n",
		ansiClearLine,
		color, icon, terminal.Reset,
		terminal.Bold, padRight(item.ManagerName, p.nameWidth), terminal.Reset,
		terminal.Dim, padRight(command, p.commandWidth), terminal.Reset,
		terminal.Dim, formatFixElapsed(item.EndedAt.Sub(item.StartedAt)), terminal.Reset,
	)
}

func (p *TTYFixProgress) writeFailureOutputLocked(item result.FixItem) {
	if strings.TrimSpace(item.Output) != "" {
		lines := capturedOutputLines(item.Output)

		p.out.PrintNewLines(1)

		indent := strings.Repeat(" ", fixFailureOutputIndent)

		for _, line := range lines {
			p.out.Printf("%s%s%s%s\n", terminal.Red+terminal.Dim, indent, line, terminal.Reset)
		}

		p.out.PrintNewLines(1)

		return
	}

	if item.Error != "" {
		p.out.Printf("%s%s%s%s\n",
			terminal.Red, strings.Repeat(" ", ttyProjectBodySpaces),
			item.Error, terminal.Reset,
		)
	}
}

func (p *TTYFixProgress) writeResultsHeader() {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	p.out.PrintNewLines(1)
	p.out.Println(terminal.Bold + "Results" + terminal.Reset)
	p.out.Println(terminal.Gray + strings.Repeat("─", fixResultsRuleWidth) + terminal.Reset)
}

func fixProgressCommandLabel(candidate engine.FixCandidate) string {
	if candidate.Command != "" {
		return candidate.Command
	}

	return fixProgressSpinnerLabel
}
