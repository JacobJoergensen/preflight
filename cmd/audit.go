package cmd

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/render"
)

type auditFlags struct {
	scanFlags
	minSeverity string
}

func newAuditCommand() *cobra.Command {
	flags := &auditFlags{}

	cmd := &cobra.Command{
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
preflight audit --only js,composer
preflight audit -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, err := parseFormat(flags.format)
			if err != nil {
				return err
			}

			return runScan(cmd, scanCommand[result.AuditReport]{
				failMsg: "audit failed",
				timeout: flags.timeout,
				run: func(ctx context.Context, runner engine.Runner, profile config.Profile) (result.AuditReport, error) {
					var onlyProfile *[]string
					var minSeverityProfile *string

					if profile.Audit != nil {
						onlyProfile = profile.Audit.Only
						minSeverityProfile = profile.Audit.MinSeverity
					}

					only := flagOrProfile(cmd, "only", flags.only, onlyProfile)
					minSeverity := flagOrProfile(cmd, "min-severity", flags.minSeverity, minSeverityProfile)

					progress := buildScanProgress(jsonOut, "auditing…")
					defer progress.Close()

					return runner.Audit(ctx, only, minSeverity, progress, flags.noMonorepo, flags.projectGlobs)
				},
				render: func(report result.AuditReport) error { return renderAudit(report, jsonOut) },
				markdown: func(report result.AuditReport, w io.Writer) error {
					return render.MarkdownAuditRenderer{Out: w}.Render(report)
				},
				exitCode: func(report result.AuditReport) int {
					return reportExitCode(report.Canceled, report.Items, func(item result.AuditItem) bool {
						return item.ErrText != "" || !item.OK
					})
				},
			})
		},
	}

	registerScanFlags(cmd, &flags.scanFlags, 30*time.Minute, "audit processes")
	cmd.Flags().StringVar(&flags.minSeverity, "min-severity", "", "Minimum severity to report (info, low, moderate, high, critical)")

	return cmd
}

func renderAudit(report result.AuditReport, jsonOutput bool) error {
	if jsonOutput {
		return render.JSONAuditRenderer{Out: os.Stdout}.Render(report)
	}

	return render.TTYAuditRenderer{}.Render(report)
}
