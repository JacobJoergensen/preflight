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

type licensesFlags struct {
	scanFlags
	allow []string
	deny  []string
}

func newLicensesCommand() *cobra.Command {
	flags := &licensesFlags{}

	cmd := &cobra.Command{
		Use:   "licenses",
		Short: "Check dependency licenses against an allow/deny policy",
		Long: `Lists the declared license of each installed dependency and flags any that violate your policy.

License sources by scope:
  • composer — composer licenses --format=json
  • rust — cargo metadata
  • js — node_modules package manifests

Define the policy in preflight.yml (licenses.allow / licenses.deny) or with --allow/--deny. A package violates when its license is denied, or (when an allowlist is set) is not on it.`,
		Example: `preflight licenses --deny GPL-3.0-only,AGPL-3.0-only
preflight licenses --allow MIT,Apache-2.0,BSD-3-Clause
preflight licenses -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, err := parseFormat(flags.format)
			if err != nil {
				return err
			}

			policyConfigured := false

			return runScan(cmd, scanCommand[result.LicenseReport]{
				failMsg: "license check failed",
				timeout: flags.timeout,
				run: func(ctx context.Context, runner engine.Runner, profile config.Profile) (result.LicenseReport, error) {
					var onlyProfile, allowProfile, denyProfile *[]string

					if profile.Licenses != nil {
						onlyProfile = profile.Licenses.Only
						allowProfile = profile.Licenses.Allow
						denyProfile = profile.Licenses.Deny
					}

					only := flagOrProfile(cmd, "only", flags.only, onlyProfile)
					allow := mergeStringList(allowProfile, flags.allow)
					deny := mergeStringList(denyProfile, flags.deny)

					policyConfigured = len(allow) > 0 || len(deny) > 0

					progress := buildScanProgress(jsonOut, "checking licenses…")
					defer progress.Close()

					return runner.Licenses(ctx, only, allow, deny, progress, flags.noMonorepo, flags.projectGlobs)
				},
				render: func(report result.LicenseReport) error { return renderLicenses(report, jsonOut, policyConfigured) },
				markdown: func(report result.LicenseReport, w io.Writer) error {
					return render.MarkdownLicenseRenderer{Out: w, PolicyConfigured: policyConfigured}.Render(report)
				},
				exitCode: func(report result.LicenseReport) int {
					return reportExitCode(report.Canceled, report.Items, func(item result.LicenseItem) bool {
						return item.ErrText != "" || len(item.Violations) > 0
					})
				},
			})
		},
	}

	registerScanFlags(cmd, &flags.scanFlags, 5*time.Minute, "license check")
	cmd.Flags().StringSliceVar(&flags.allow, "allow", nil, "Allowed SPDX license IDs (comma-separated); anything else is a violation")
	cmd.Flags().StringSliceVar(&flags.deny, "deny", nil, "Denied SPDX license IDs (comma-separated)")

	return cmd
}

func renderLicenses(report result.LicenseReport, jsonOutput, policyConfigured bool) error {
	if jsonOutput {
		return render.JSONLicenseRenderer{Out: os.Stdout}.Render(report)
	}

	return render.TTYLicenseRenderer{PolicyConfigured: policyConfigured}.Render(report)
}
