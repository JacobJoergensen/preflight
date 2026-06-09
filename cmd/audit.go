package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/release"
	"github.com/JacobJoergensen/preflight/internal/render"
)

type auditFlags struct {
	scanFlags
	minSeverity string
	ignoreCVEs  []string
}

func newAuditCommand() *cobra.Command {
	flags := &auditFlags{}

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Run native security audits (npm audit, composer audit, govulncheck, …)",
		Long: `Runs each ecosystem's native vulnerability scanner for the selected scopes. Filter with --min-severity; runs per sub-project in monorepos.

Tools used by scope:
  • js: npm/pnpm/yarn/bun audit --json
  • composer: composer audit --format=json
  • go: govulncheck -json ./... (install separately if missing)
  • python: pip-audit --format json (optional tool)
  • ruby: bundle-audit check (bundler-audit gem)

This does not replace dedicated security pipelines, it unifies invocation and reporting.`,
		Example: `preflight audit
preflight audit --only js,composer
preflight audit -o json
preflight audit -o sarif > preflight.sarif`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := parseAuditFormat(flags.format)
			if err != nil {
				return err
			}

			return runScan(cmd, scanCommand[result.AuditReport]{
				failMsg: "audit failed",
				timeout: flags.timeout,
				run: func(ctx context.Context, runner engine.Runner, profile config.Profile) (result.AuditReport, error) {
					var onlyProfile *[]string
					var minSeverityProfile *string
					var ignoredProfile *[]string

					if profile.Audit != nil {
						onlyProfile = profile.Audit.Only
						minSeverityProfile = profile.Audit.MinSeverity
						ignoredProfile = profile.Audit.IgnoredCVEs
					}

					only := flagOrProfile(cmd, "only", flags.only, onlyProfile)
					minSeverity := flagOrProfile(cmd, "min-severity", flags.minSeverity, minSeverityProfile)
					ignored := mergeStringList(ignoredProfile, flags.ignoreCVEs)

					progress := buildScanProgress(format != auditFormatText, "auditing…")
					defer progress.Close()

					return runner.Audit(ctx, only, minSeverity, ignored, progress, flags.noMonorepo, flags.projectGlobs)
				},
				render: func(report result.AuditReport) error { return renderAudit(report, format) },
				markdown: func(report result.AuditReport, w io.Writer) error {
					return render.MarkdownAuditRenderer{Out: w}.Render(report)
				},
				exitCode: func(report result.AuditReport) int {
					// In SARIF mode findings are reported to code scanning, not via the
					// exit code, so the upload step still runs; tool errors still fail.
					failed := func(item result.AuditItem) bool {
						if item.Skipped {
							return false
						}

						if format == auditFormatSARIF {
							return item.ErrText != ""
						}

						return item.ErrText != "" || !item.OK
					}

					return reportExitCode(report.Canceled, report.Items, failed)
				},
			})
		},
	}

	registerScanFlags(cmd, &flags.scanFlags, 30*time.Minute, "audit processes")
	cmd.Flags().Lookup("format").Usage = "Output format: text, json, or sarif"
	cmd.Flags().StringVar(&flags.minSeverity, "min-severity", "", "Minimum severity to report (info, low, moderate, high, critical)")
	cmd.Flags().StringArrayVar(&flags.ignoreCVEs, "ignore-cve", nil, "Advisory ID (CVE/GHSA/…) to suppress; repeatable, merged with preflight.yml ignoredCves")

	return cmd
}

const (
	auditFormatText  = "text"
	auditFormatJSON  = "json"
	auditFormatSARIF = "sarif"
)

func parseAuditFormat(format string) (string, error) {
	switch format {
	case "", auditFormatText:
		return auditFormatText, nil
	case auditFormatJSON:
		return auditFormatJSON, nil
	case auditFormatSARIF:
		return auditFormatSARIF, nil
	default:
		return "", fmt.Errorf("invalid --format %q (use text, json, or sarif)", format)
	}
}

func mergeStringList(profile *[]string, flag []string) []string {
	var merged []string

	if profile != nil {
		merged = append(merged, *profile...)
	}

	return append(merged, flag...)
}

func renderAudit(report result.AuditReport, format string) error {
	switch format {
	case auditFormatJSON:
		return render.JSONAuditRenderer{Out: os.Stdout}.Render(report)
	case auditFormatSARIF:
		return render.SARIFAuditRenderer{Out: os.Stdout, ToolVersion: release.Version}.Render(report)
	default:
		return render.TTYAuditRenderer{}.Render(report)
	}
}
