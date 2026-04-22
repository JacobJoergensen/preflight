package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/render"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type auditOptions struct {
	managers     []string
	scopes       []string
	json         bool
	timeout      time.Duration
	minSeverity  string
	noMonorepo   bool
	projectGlobs []string
}

var auditOpts auditOptions

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Run native security audits (npm audit, composer audit, govulncheck, …)",
	Long: `Runs each ecosystem's native vulnerability scanner for the selected scopes. Filter with --min-severity; runs per sub-project in monorepos.

Tools used by scope:
  • js — npm/pnpm/yarn/bun audit --json
  • composer — composer audit --format=json
  • go — govulncheck -json ./... (install separately if missing)
  • python — pip-audit --format json (optional tool)
  • ruby — bundle-audit check (bundler-audit gem)

This does not replace dedicated security pipelines, it unifies invocation and reporting.`,
	Example: `preflight audit
preflight audit --scope js,composer
preflight audit --json`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), auditOpts.timeout)
		defer cancel()

		workDir, err := os.Getwd()

		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		runner := engine.NewRunner(workDir)

		config, profileName, err := loadPreflightConfig(workDir)

		if err != nil {
			return fmt.Errorf("%saudit failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		profile, err := config.ProfileFor(profileName)

		if err != nil {
			return fmt.Errorf("%s%w%s", terminal.Red, err, terminal.Reset)
		}

		var profileScope, profilePM *[]string
		minSeverity := auditOpts.minSeverity

		if profile.Audit != nil {
			profileScope = profile.Audit.Scope
			profilePM = profile.Audit.PM

			if minSeverity == "" && profile.Audit.MinSeverity != nil {
				minSeverity = *profile.Audit.MinSeverity
			}
		}

		scopes, managers := resolveScopeAndPM(cmd, auditOpts.scopes, auditOpts.managers, profileScope, profilePM)

		if err := validateScopeAndPM(scopes, managers); err != nil {
			return err
		}

		progress := buildAuditProgress(auditOpts.json)

		report, err := runner.Audit(ctx, scopes, managers, minSeverity, progress, auditOpts.noMonorepo, auditOpts.projectGlobs)

		progress.Close()

		if err != nil {
			return fmt.Errorf("%saudit failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		if err := renderAudit(report, auditOpts.json); err != nil {
			return err
		}

		writeGitHubSummary(func(w io.Writer) error {
			return render.MarkdownAuditRenderer{Out: w}.Render(report)
		})

		if exitCodeFromAuditReport(report) != 0 {
			return ErrSilentFailure
		}

		return nil
	},
}

func renderAudit(report result.AuditReport, jsonOutput bool) error {
	if jsonOutput {
		return render.JSONAuditRenderer{Out: os.Stdout}.Render(report)
	}

	return render.TTYAuditRenderer{}.Render(report)
}

func buildAuditProgress(jsonOutput bool) engine.AuditProgress {
	if jsonOutput || terminal.Quiet {
		return engine.NoopAuditProgress{}
	}

	if !terminal.IsInteractiveTTY(os.Stdout) {
		return engine.NoopAuditProgress{}
	}

	return render.NewTTYProgress(os.Stdout, "auditing…")
}

func exitCodeFromAuditReport(report result.AuditReport) int {
	if report.Canceled {
		return 1
	}

	for _, item := range report.Items {
		if item.ErrText != "" {
			return 1
		}

		if !item.OK {
			return 1
		}
	}

	return 0
}

func init() {
	auditCmd.Flags().StringSliceVarP(
		&auditOpts.managers,
		"pm",
		"p",
		[]string{},
		"Tools or scopes to audit (aliases: npm,yarn,pnpm,bun → js)",
	)

	auditCmd.Flags().StringSliceVar(
		&auditOpts.scopes,
		"scope",
		[]string{},
		"Scopes to audit (comma-separated: js,composer,go,python,ruby)",
	)

	auditCmd.Flags().DurationVarP(
		&auditOpts.timeout,
		"timeout",
		"t",
		30*time.Minute,
		"Timeout for all audit processes",
	)

	auditCmd.Flags().BoolVar(
		&auditOpts.json,
		"json",
		false,
		"Output results as JSON",
	)

	auditCmd.Flags().StringVar(
		&auditOpts.minSeverity,
		"min-severity",
		"",
		"Minimum severity to report (info, low, moderate, high, critical)",
	)

	auditCmd.Flags().BoolVar(
		&auditOpts.noMonorepo,
		"no-monorepo",
		false,
		"Disable monorepo traversal, audit only the current directory",
	)

	auditCmd.Flags().StringSliceVar(
		&auditOpts.projectGlobs,
		"project",
		[]string{},
		"Restrict monorepo traversal to projects matching these path globs (comma-separated, e.g. packages/*)",
	)

	rootCmd.AddCommand(auditCmd)
}
