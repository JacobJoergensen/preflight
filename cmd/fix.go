package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/render"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type fixOptions struct {
	force        bool
	timeout      time.Duration
	managers     []string
	scopes       []string
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
	Use:   "fix",
	Short: "Fix missing dependencies across multiple package managers",
	Long: `Fix command installs and reconciles dependencies for your project.
Supports Composer, NPM, PNPM, Yarn, Bun, and Go modules.
Prompts per ecosystem by default; use --yes to apply without asking.
Example: preflight fix --pm=npm,composer`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), fixOpts.timeout)
		defer cancel()

		workDir, err := os.Getwd()

		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		runner := engine.NewRunner(workDir)

		config, profName, err := loadPreflightConfig(workDir)

		if err != nil {
			return fmt.Errorf("%sfix failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		profile, err := config.ProfileFor(profName)

		if err != nil {
			return fmt.Errorf("%s%w%s", terminal.Red, err, terminal.Reset)
		}

		var profileScope, profilePM *[]string

		if profile.Fix != nil {
			profileScope = profile.Fix.Scope
			profilePM = profile.Fix.PM
		}

		scopes, managers := resolveScopeAndPM(cmd, fixOpts.scopes, fixOpts.managers, profileScope, profilePM)

		if err := validateScopeAndPM(scopes, managers); err != nil {
			return err
		}

		approver := buildFixApprover(fixOpts)
		progress, itemsRenderedLive := buildFixProgress(fixOpts)

		report, err := runner.Fix(ctx, scopes, managers, adapter.FixOptions{
			Force:      fixOpts.force,
			SkipBackup: fixOpts.skipBackup,
			DryRun:     fixOpts.dryRun,
		}, !fixOpts.noDiff, approver, progress, fixOpts.noMonorepo, fixOpts.projectGlobs)

		if err != nil {
			return fmt.Errorf("%sfix failed: %w%s", terminal.Red, err, terminal.Reset)
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
	if opts.dryRun || opts.json || terminal.Quiet {
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
	fixCmd.Flags().StringSliceVarP(&fixOpts.managers, "pm", "p", []string{}, "Tools or scopes to fix (aliases: npm,yarn,pnpm,bun)")
	fixCmd.Flags().StringSliceVar(&fixOpts.scopes, "scope", []string{}, "Scopes to fix (comma-separated: js,composer,go,python,ruby)")
	fixCmd.Flags().BoolVar(&fixOpts.skipBackup, "skip-backup", false, "Skip creating backup of lock files")
	fixCmd.Flags().BoolVar(&fixOpts.dryRun, "dry-run", false, "Show what would be done without making changes")
	fixCmd.Flags().BoolVar(&fixOpts.noDiff, "no-diff", false, "Hide per-package version changes from lock files")
	fixCmd.Flags().BoolVarP(&fixOpts.assumeYes, "yes", "y", false, "Apply every ecosystem without prompting")
	fixCmd.Flags().BoolVar(&fixOpts.json, "json", false, "Output results as JSON")
	fixCmd.Flags().BoolVar(&fixOpts.noMonorepo, "no-monorepo", false, "Disable monorepo traversal, fix only the current directory")
	fixCmd.Flags().StringSliceVar(&fixOpts.projectGlobs, "project", []string{}, "Restrict monorepo traversal to projects matching these path globs (comma-separated, e.g. packages/*)")

	rootCmd.AddCommand(fixCmd)
}
