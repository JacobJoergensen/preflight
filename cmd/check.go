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
	managers []string
	scopes   []string
	withEnv  bool
	timeout  time.Duration
	json     bool
	outdated bool
}

var checkOpts checkOptions

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Checks if all required dependencies are installed",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), checkOpts.timeout)
		defer cancel()

		workDir, err := os.Getwd()

		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		runner := engine.NewRunner(workDir)

		config, profName, err := loadPreflightConfig(workDir)

		if err != nil {
			return fmt.Errorf("%scheck failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		profile, err := config.ProfileFor(profName)

		if err != nil {
			return fmt.Errorf("%s%w%s", terminal.Red, err, terminal.Reset)
		}

		var profileScope, profilePM *[]string
		withEnv := checkOpts.withEnv

		if profile.Check != nil {
			profileScope = profile.Check.Scope
			profilePM = profile.Check.PM

			if !cmd.Flags().Changed("with-env") && profile.Check.WithEnv != nil {
				withEnv = *profile.Check.WithEnv
			}
		}

		scopes, managers := resolveScopeAndPM(cmd, checkOpts.scopes, checkOpts.managers, profileScope, profilePM)

		if err := validateScopeAndPM(scopes, managers); err != nil {
			return err
		}

		report, err := runner.Check(ctx, scopes, managers, withEnv, checkOpts.outdated)

		if err != nil {
			return fmt.Errorf("%scheck failed: %w%s", terminal.Red, err, terminal.Reset)
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

func exitCodeFromReport(report result.CheckReport) int {
	if report.Canceled {
		return 1
	}

	for _, item := range report.Items {
		if len(item.Errors) > 0 {
			return 1
		}
	}

	return 0
}

func init() {
	checkCmd.Flags().StringSliceVarP(
		&checkOpts.managers,
		"pm",
		"p",
		[]string{},
		"Tools or scopes to check (aliases: npm,yarn,pnpm,bun → js, use `env` for .env validation)",
	)

	checkCmd.Flags().StringSliceVar(
		&checkOpts.scopes,
		"scope",
		[]string{},
		"Scopes to check (comma-separated: js,php,composer,node,go,python,ruby,env)",
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

	rootCmd.AddCommand(checkCmd)
}
