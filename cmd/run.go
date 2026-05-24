package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/run"
)

func newRunCommand() *cobra.Command {
	var (
		dryRun  bool
		timeout time.Duration
	)

	cmd := &cobra.Command{
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

			workDir, err := workingDir()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			cfg, profileName, err := loadPreflightConfig(workDir)
			if err != nil {
				return err
			}

			profile, err := cfg.ProfileFor(profileName)
			if err != nil {
				return err
			}

			if profile.Run == nil || len(profile.Run.Scripts) == 0 {
				return fmt.Errorf("no run.scripts for profile %q in preflight.yml (configure profiles.%s.run.scripts)",
					profileName, profileName)
			}

			targets, ok := profile.Run.Scripts[alias]

			if !ok {
				keys := make([]string, 0, len(profile.Run.Scripts))

				for key := range profile.Run.Scripts {
					keys = append(keys, key)
				}

				slices.Sort(keys)

				return fmt.Errorf("unknown script %q for profile %q, known: %s",
					alias, profileName, strings.Join(keys, ", "))
			}

			rc := ecosystem.RunContext{WorkDir: workDir, FS: fs.OSFS{}}

			scripts, err := run.ResolveScripts(rc, targets)
			if err != nil {
				return err
			}

			if dryRun {
				for _, script := range scripts {
					line := script.Bin + " " + strings.Join(script.Args, " ")

					if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
						return fmt.Errorf("write stdout: %w", err)
					}
				}

				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			for _, script := range scripts {
				if err := ctx.Err(); err != nil {
					return err
				}

				if _, err := exec.RunStreamingInDir(ctx, workDir, script.Bin, script.Args, os.Stdout, os.Stderr); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the resolved command without running it")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", 30*time.Minute, "Timeout for the script process")

	return cmd
}
