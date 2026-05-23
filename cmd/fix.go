package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/render"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type fixOptions struct {
	force        bool
	timeout      time.Duration
	only         []string
	skipBackup   bool
	dryRun       bool
	noDiff       bool
	assumeYes    bool
	json         bool
	noMonorepo   bool
	projectGlobs []string
}

var fixOpts fixOptions

var fixCmd = &cobra.Command{
	Use:     "fix",
	Short:   "Fix missing dependencies across multiple package managers",
	Long:    `Installs missing dependencies across Composer, NPM, PNPM, Yarn, Bun, and Go. Interactive by default (--yes to auto-approve), prints a lock file diff per step (--no-diff to hide), and runs per sub-project in monorepos.`,
	Example: "preflight fix --pm=npm,composer",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), fixOpts.timeout)
		defer cancel()

		runner, profile, err := commandSetup("fix failed")
		if err != nil {
			return err
		}

		var profileOnly *[]string

		if profile.Fix != nil {
			profileOnly = profile.Fix.Only
		}

		only := resolveOnly(cmd, fixOpts.only, profileOnly)

		approver := buildFixApprover(fixOpts)
		progress, itemsRenderedLive := buildFixProgress(fixOpts)

		report, err := runner.Fix(ctx, only, ecosystem.FixOptions{
			Force:      fixOpts.force,
			SkipBackup: fixOpts.skipBackup,
			DryRun:     fixOpts.dryRun,
		}, !fixOpts.noDiff, approver, progress, fixOpts.noMonorepo, fixOpts.projectGlobs)
		if err != nil {
			return fmt.Errorf("fix failed: %w", err)
		}

		if err := renderFix(report, fixOpts.json, itemsRenderedLive); err != nil {
			return err
		}

		writeGitHubSummary(func(w io.Writer) error {
			return render.MarkdownFixRenderer{Out: w}.Render(report)
		})

		if exitCodeFromFixReport(report) != 0 {
			return ErrSilentFailure
		}

		return nil
	},
}

func buildFixApprover(opts fixOptions) engine.FixApprover {
	if opts.assumeYes || opts.dryRun || opts.json || terminal.Quiet {
		return engine.AutoFixApprover{}
	}

	if !terminal.IsInteractiveTTY(os.Stdin) || !terminal.IsInteractiveTTY(os.Stdout) {
		return engine.AutoFixApprover{}
	}

	return render.NewTTYFixApprover(os.Stdin, os.Stdout)
}

func buildFixProgress(opts fixOptions) (engine.FixProgress, bool) {
	if opts.dryRun || opts.json || terminal.Quiet || rootOpts.debug {
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

func exitCodeFromFixReport(report result.FixReport) int {
	if report.Canceled || report.Aborted {
		return 1
	}

	for _, item := range report.Items {
		if !item.Success {
			return 1
		}
	}

	return 0
}

func init() {
	fixCmd.Flags().BoolVarP(&fixOpts.force, "force", "f", false, "Force reinstall dependencies")
	fixCmd.Flags().DurationVarP(&fixOpts.timeout, "timeout", "t", 30*time.Minute, "Timeout for fix operation")
	fixCmd.Flags().StringSliceVar(&fixOpts.only, "only", []string{}, "Limit to these ecosystems or tools (comma-separated: js, npm, composer, go, rust, python, ruby)")
	fixCmd.Flags().BoolVar(&fixOpts.skipBackup, "skip-backup", false, "Skip creating backup of lock files")
	fixCmd.Flags().BoolVar(&fixOpts.dryRun, "dry-run", false, "Show what would be done without making changes")
	fixCmd.Flags().BoolVar(&fixOpts.noDiff, "no-diff", false, "Hide per-package version changes from lock files")
	fixCmd.Flags().BoolVarP(&fixOpts.assumeYes, "yes", "y", false, "Apply every ecosystem without prompting")
	fixCmd.Flags().BoolVar(&fixOpts.json, "json", false, "Output results as JSON")
	fixCmd.Flags().BoolVar(&fixOpts.noMonorepo, "no-monorepo", false, "Disable monorepo traversal, fix only the current directory")
	fixCmd.Flags().StringSliceVar(&fixOpts.projectGlobs, "project", []string{}, "Restrict monorepo traversal to projects matching these path globs (comma-separated, e.g. packages/*)")

	rootCmd.AddCommand(fixCmd)
}
