package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/render"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type auditOptions struct {
	managers []string
	scopes   []string
	json     bool
	timeout  time.Duration
}

var auditOpts auditOptions

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Run native security audits (npm audit, composer audit, govulncheck, …)",
	Long: `Runs each ecosystem's native vulnerability scanner for the selected scopes.

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

		workDir, _ := os.Getwd()
		runner := engine.NewRunner(workDir)

		if len(auditOpts.scopes) > 0 && len(auditOpts.managers) > 0 {
			return fmt.Errorf("%scannot use both --scope and --pm%s", terminal.Red, terminal.Reset)
		}

		report, err := runner.Audit(ctx, auditOpts.scopes, auditOpts.managers)

		if err != nil {
			return fmt.Errorf("%saudit failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		if err := renderAudit(report, auditOpts.json); err != nil {
			return err
		}

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

func exitCodeFromAuditReport(report result.AuditReport) int {
	if report.Canceled {
		return 1
	}

	for _, item := range report.Items {
		if item.Skipped {
			continue
		}

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

	rootCmd.AddCommand(auditCmd)
}
