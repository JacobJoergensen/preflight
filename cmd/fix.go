package cmd

import (
	"context"
	"fmt"
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
	force      bool
	timeout    time.Duration
	managers   []string
	scopes     []string
	skipBackup bool
	dryRun     bool
	json       bool
}

var fixOpts fixOptions

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Fix missing dependencies across multiple package managers",
	Long: `Fix command installs and repairs missing dependencies for your project.
Supports Composer, NPM, PNPM, Yarn, Bun, and Go modules.
Example: preflight fix --pm=npm,composer`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), fixOpts.timeout)
		defer cancel()

		workDir, _ := os.Getwd()
		runner := engine.NewRunner(workDir)

		config, profName, err := loadPreflightConfig(workDir)

		if err != nil {
			return fmt.Errorf("%sfix failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		profile, err := config.ProfileFor(profName)

		if err != nil {
			return fmt.Errorf("%s%w%s", terminal.Red, err, terminal.Reset)
		}

		scopes := fixOpts.scopes
		managers := fixOpts.managers

		scopeFromCLI := cmd.Flags().Changed("scope")
		pmFromCLI := cmd.Flags().Changed("pm")

		if profile.Fix != nil {
			fix := profile.Fix

			if !scopeFromCLI && !pmFromCLI && fix.Scope != nil {
				scopes = *fix.Scope
			}

			if !scopeFromCLI && !pmFromCLI && fix.PM != nil {
				managers = *fix.PM
			}
		}

		if len(scopes) > 0 && len(managers) > 0 {
			return fmt.Errorf("%scannot use both --scope and --pm%s", terminal.Red, terminal.Reset)
		}

		report, err := runner.Fix(ctx, scopes, managers, adapter.FixOptions{
			Force:      fixOpts.force,
			SkipBackup: fixOpts.skipBackup,
			DryRun:     fixOpts.dryRun,
		})

		if err != nil {
			return fmt.Errorf("%sfix failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		if err := renderFix(report, fixOpts.json); err != nil {
			return err
		}

		if exitCodeFromFixReport(report) != 0 {
			return ErrSilentFailure
		}

		return nil
	},
}

func renderFix(report result.FixReport, jsonOutput bool) error {
	if jsonOutput {
		return render.JSONFixRenderer{Out: os.Stdout}.Render(report)
	}

	return render.TTYFixRenderer{}.Render(report)
}

func exitCodeFromFixReport(report result.FixReport) int {
	if report.Canceled || report.InternalError != "" {
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
	fixCmd.Flags().BoolVar(&fixOpts.json, "json", false, "Output results as JSON")

	rootCmd.AddCommand(fixCmd)
}
