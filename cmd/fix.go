package cmd

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/render"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type fixFlags struct {
	scanFlags
	force      bool
	skipBackup bool
	dryRun     bool
	noDiff     bool
	assumeYes  bool
}

func newFixCommand() *cobra.Command {
	flags := &fixFlags{}

	cmd := &cobra.Command{
		Use:     "fix",
		Short:   "Fix missing dependencies across multiple package managers",
		Long:    `Installs missing dependencies across Composer, NPM, PNPM, Yarn, Bun, and Go. Interactive by default (--yes to auto-approve), prints a lock file diff per step (--no-diff to hide), and runs per sub-project in monorepos.`,
		Example: "preflight fix --only npm,composer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, err := parseFormat(flags.format)
			if err != nil {
				return err
			}

			approver := buildFixApprover(flags, jsonOut)
			progress, itemsRenderedLive := buildFixProgress(flags, jsonOut)

			return runScan(cmd, scanCommand[result.FixReport]{
				failMsg: "fix failed",
				timeout: flags.timeout,
				run: func(ctx context.Context, runner engine.Runner, profile config.Profile) (result.FixReport, error) {
					var onlyProfile *[]string

					if profile.Fix != nil {
						onlyProfile = profile.Fix.Only
					}

					only := flagOrProfile(cmd, "only", flags.only, onlyProfile)

					return runner.Fix(ctx, only, ecosystem.FixOptions{
						Force:      flags.force,
						SkipBackup: flags.skipBackup,
						DryRun:     flags.dryRun,
					}, !flags.noDiff, approver, progress, flags.noMonorepo, flags.projectGlobs)
				},
				render: func(report result.FixReport) error { return renderFix(report, jsonOut, itemsRenderedLive) },
				markdown: func(report result.FixReport, w io.Writer) error {
					return render.MarkdownFixRenderer{Out: w}.Render(report)
				},
				exitCode: func(report result.FixReport) int {
					return reportExitCode(report.Canceled || report.Aborted, report.Items, func(item result.FixItem) bool {
						return !item.Success
					})
				},
			})
		},
	}

	registerScanFlags(cmd, &flags.scanFlags, 30*time.Minute, "the fix operation")
	cmd.Flags().BoolVarP(&flags.force, "force", "f", false, "Force reinstall dependencies")
	cmd.Flags().BoolVar(&flags.skipBackup, "skip-backup", false, "Skip creating backup of lock files")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Show what would be done without making changes")
	cmd.Flags().BoolVar(&flags.noDiff, "no-diff", false, "Hide per-package version changes from lock files")
	cmd.Flags().BoolVarP(&flags.assumeYes, "yes", "y", false, "Apply every ecosystem without prompting")

	return cmd
}

func buildFixApprover(flags *fixFlags, jsonOutput bool) engine.FixApprover {
	if flags.assumeYes || flags.dryRun || jsonOutput || terminal.Quiet {
		return engine.AutoFixApprover{}
	}

	if !terminal.IsInteractiveTTY(os.Stdin) || !terminal.IsInteractiveTTY(os.Stdout) {
		return engine.AutoFixApprover{}
	}

	return render.NewTTYFixApprover(os.Stdin, os.Stdout)
}

func buildFixProgress(flags *fixFlags, jsonOutput bool) (engine.FixProgress, bool) {
	if flags.dryRun || jsonOutput || terminal.Quiet || rootOpts.debug {
		return engine.NoopFixProgress{}, false
	}

	if !terminal.IsInteractiveTTY(os.Stdout) {
		return engine.NoopFixProgress{}, false
	}

	return render.NewTTYFixProgress(os.Stdout), true
}

func renderFix(report result.FixReport, jsonOutput, skipItems bool) error {
	if jsonOutput {
		return render.JSONFixRenderer{Out: os.Stdout}.Render(report)
	}

	return render.TTYFixRenderer{SkipItems: skipItems}.Render(report)
}
