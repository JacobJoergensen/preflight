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
