package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/manifest"
	"github.com/JacobJoergensen/preflight/internal/run"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type runOptions struct {
	dryRun  bool
	timeout time.Duration
}

var runOpts runOptions

var runCmd = &cobra.Command{
	Use:   "run [script]",
	Short: "Run a named script from preflight.yml (profiles.*.run.scripts)",
	Long: `Runs a script mapped under the active profile's run.scripts table.

This is orchestration on top of check/fix: each script selects exactly one
package manager target (js, composer, go, ruby, or python) so resolution is never ambiguous.

Requires preflight.yml. Use native npm, composer, etc. directly for ad-hoc commands.`,
	Example: `preflight run test
preflight run build --profile ci`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]

		workDir, err := os.Getwd()

		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		config, profileName, err := loadPreflightConfig(workDir)

		if err != nil {
			return err
		}

		profile, err := config.ProfileFor(profileName)

		if err != nil {
			return err
		}

		if profile.Run == nil || len(profile.Run.Scripts) == 0 {
			return fmt.Errorf("%sno run.scripts for profile %q in preflight.yml (configure profiles.%s.run.scripts)%s",
				terminal.Red, profileName, profileName, terminal.Reset)
		}

		targets, ok := profile.Run.Scripts[alias]

		if !ok {
			keys := make([]string, 0, len(profile.Run.Scripts))

			for key := range profile.Run.Scripts {
				keys = append(keys, key)
			}

			slices.Sort(keys)

			return fmt.Errorf("%sunknown script %q for profile %q, known: %s%s",
				terminal.Red, alias, profileName, strings.Join(keys, ", "), terminal.Reset)
		}

		loader := manifest.NewLoader(workDir)

		scripts, err := run.ResolveScripts(loader, targets)

		if err != nil {
			return fmt.Errorf("%s%w%s", terminal.Red, err, terminal.Reset)
		}

		if runOpts.dryRun {
			for _, s := range scripts {
				line := s.Bin + " " + strings.Join(s.Args, " ")

				if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
					return fmt.Errorf("write stdout: %w", err)
				}
			}

			return nil
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), runOpts.timeout)
		defer cancel()

		for _, s := range scripts {
			if err := exec.RunStreamingInDir(ctx, workDir, s.Bin, s.Args, os.Stdout, os.Stderr); err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	runCmd.Flags().BoolVar(&runOpts.dryRun, "dry-run", false, "Print the resolved command without running it")
	runCmd.Flags().DurationVar(&runOpts.timeout, "timeout", 30*time.Minute, "Timeout for the script process")

	rootCmd.AddCommand(runCmd)
}
