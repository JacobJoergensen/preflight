package cmd

import (
	"context"
	"fmt"
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

type checkFlags struct {
	scanFlags
	withEnv  bool
	outdated bool
}

func newCheckCommand() *cobra.Command {
	flags := &checkFlags{}

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Checks if all required dependencies are installed",
		Long:  `Validates installed dependencies for the selected scopes. Supports monorepo traversal, .env validation (--with-env), and outdated package reporting (--outdated).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, err := parseFormat(flags.format)
			if err != nil {
				return err
			}

			var resolvedOnly []string

			return runScan(cmd, scanCommand[result.CheckReport]{
				failMsg: "check failed",
				timeout: flags.timeout,
				run: func(ctx context.Context, runner engine.Runner, profile config.Profile) (result.CheckReport, error) {
					var onlyProfile *[]string
					var withEnvProfile *bool

					if profile.Check != nil {
						onlyProfile = profile.Check.Only
						withEnvProfile = profile.Check.WithEnv
					}

					only := flagOrProfile(cmd, "only", flags.only, onlyProfile)
					resolvedOnly = only
					withEnv := flagOrProfile(cmd, "with-env", flags.withEnv, withEnvProfile)

					progress := buildScanProgress(jsonOut, "checking…")
					defer progress.Close()

					return runner.Check(ctx, only, withEnv, flags.outdated, progress, flags.noMonorepo, flags.projectGlobs)
				},
				render: func(report result.CheckReport) error { return renderCheck(report, jsonOut) },
				markdown: func(report result.CheckReport, w io.Writer) error {
					return render.MarkdownCheckRenderer{Out: w}.Render(report)
				},
				exitCode: func(report result.CheckReport) int {
					return reportExitCode(report.Canceled, report.Items, func(item result.CheckItem) bool {
						return len(item.Errors()) > 0
					})
				},
				afterRender: func(report result.CheckReport) (bool, error) {
					return offerFix(cmd, report, jsonOut, resolvedOnly, flags.noMonorepo, flags.projectGlobs)
				},
			})
		},
	}

	registerScanFlags(cmd, &flags.scanFlags, 5*time.Minute, "checks")
	cmd.Flags().BoolVar(&flags.withEnv, "with-env", false, "Also validate `.env` against `.env.example` (in addition to selected dependency checks)")
	cmd.Flags().BoolVar(&flags.outdated, "outdated", false, "Also check for outdated packages")

	return cmd
}

func renderCheck(report result.CheckReport, jsonOutput bool) error {
	if jsonOutput {
		return render.JSONCheckRenderer{Out: os.Stdout}.Render(report)
	}

	return render.TTYCheckRenderer{}.Render(report)
}

func offerFix(cmd *cobra.Command, report result.CheckReport, jsonOutput bool, only []string, noMonorepo bool, projectGlobs []string) (bool, error) {
	if jsonOutput || terminal.Quiet || !terminal.IsInteractive() {
		return false, nil
	}

	count := checkErrorCount(report)
	if count == 0 {
		return false, nil
	}

	noun := "issue"
	if count != 1 {
		noun = "issues"
	}

	if _, err := fmt.Fprintln(os.Stdout); err != nil {
		return false, err
	}

	run, err := terminal.Ask(os.Stdin, os.Stdout, fmt.Sprintf("Run preflight fix to resolve %d %s?", count, noun))
	if err != nil || !run {
		return false, err
	}

	return runFixFromCheck(cmd, only, noMonorepo, projectGlobs)
}

func checkErrorCount(report result.CheckReport) int {
	count := 0

	for _, item := range report.Items {
		count += render.CountMessageItems(item.Errors())
	}

	return count
}

func runFixFromCheck(cmd *cobra.Command, only []string, noMonorepo bool, projectGlobs []string) (bool, error) {
	runner, _, err := commandSetup("fix failed")
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
	defer cancel()

	approver := render.NewTTYFixApprover(os.Stdin, os.Stdout)
	progress := render.NewTTYFixProgress(os.Stdout)

	report, err := runner.Fix(ctx, only, ecosystem.FixOptions{}, true, approver, progress, noMonorepo, projectGlobs)
	if err != nil {
		return false, err
	}

	if err := renderFix(report, false, true); err != nil {
		return false, err
	}

	if report.Canceled || report.Aborted {
		return false, nil
	}

	for _, item := range report.Items {
		if !item.Success {
			return false, nil
		}
	}

	return true, nil
}
