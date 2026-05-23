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

type checkOptions struct {
	only         []string
	withEnv      bool
	timeout      time.Duration
	json         bool
	outdated     bool
	noMonorepo   bool
	projectGlobs []string
}

var checkOpts checkOptions

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Checks if all required dependencies are installed",
	Long:  `Validates installed dependencies for the selected scopes. Supports monorepo traversal, .env validation (--with-env), and outdated package reporting (--outdated).`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), checkOpts.timeout)
		defer cancel()

		runner, profile, err := commandSetup("check failed")
		if err != nil {
			return err
		}

		var profileOnly *[]string
		withEnv := checkOpts.withEnv

		if profile.Check != nil {
			profileOnly = profile.Check.Only

			if !cmd.Flags().Changed("with-env") && profile.Check.WithEnv != nil {
				withEnv = *profile.Check.WithEnv
			}
		}

		only := resolveOnly(cmd, checkOpts.only, profileOnly)

		progress := buildScanProgress(checkOpts.json, "checking…")

		report, err := runner.Check(ctx, only, withEnv, checkOpts.outdated, progress, checkOpts.noMonorepo, checkOpts.projectGlobs)

		progress.Close()

		if err != nil {
			return fmt.Errorf("check failed: %w", err)
		}

		if err := renderCheck(report, checkOpts.json); err != nil {
			return err
		}

		writeGitHubSummary(func(w io.Writer) error {
			return render.MarkdownCheckRenderer{Out: w}.Render(report)
		})

		if exitCodeFromReport(report) != 0 {
			return ErrSilentFailure
		}

		return nil
	},
}

func renderCheck(report result.CheckReport, jsonOutput bool) error {
	if jsonOutput {
		return render.JSONCheckRenderer{Out: os.Stdout}.Render(report)
	}

	return render.TTYCheckRenderer{}.Render(report)
}

func buildScanProgress(jsonOutput bool, label string) engine.ScanProgress {
	if jsonOutput || terminal.Quiet || rootOpts.debug {
		return engine.NoopScanProgress{}
	}

	if !terminal.IsInteractiveTTY(os.Stdout) {
		return engine.NoopScanProgress{}
	}

	return render.NewTTYProgress(os.Stdout, label)
}

func exitCodeFromReport(report result.CheckReport) int {
	if report.Canceled {
		return 1
	}

	for _, item := range report.Items {
		if len(item.Errors()) > 0 {
			return 1
		}
	}

	return 0
}

func init() {
	checkCmd.Flags().StringSliceVar(
		&checkOpts.only,
		"only",
		[]string{},
		"Limit to these ecosystems or tools (comma-separated: js, npm, php, composer, node, go, rust, python, ruby, env)",
	)

	checkCmd.Flags().BoolVar(
		&checkOpts.withEnv,
		"with-env",
		false,
		"Also validate `.env` against `.env.example` (in addition to selected dependency checks)",
	)

	checkCmd.Flags().DurationVarP(
		&checkOpts.timeout,
		"timeout",
		"t",
		5*time.Minute,
		"Timeout for all checks to complete",
	)

	checkCmd.Flags().BoolVar(
		&checkOpts.json,
		"json",
		false,
		"Output results as JSON",
	)

	checkCmd.Flags().BoolVar(
		&checkOpts.outdated,
		"outdated",
		false,
		"Also check for outdated packages",
	)

	checkCmd.Flags().BoolVar(
		&checkOpts.noMonorepo,
		"no-monorepo",
		false,
		"Disable monorepo traversal, check only the current directory",
	)

	checkCmd.Flags().StringSliceVar(
		&checkOpts.projectGlobs,
		"project",
		[]string{},
		"Restrict monorepo traversal to projects matching these path globs (comma-separated, e.g. packages/*)",
	)

	rootCmd.AddCommand(checkCmd)
}
