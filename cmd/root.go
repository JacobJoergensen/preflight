package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/terminal"
)

var ErrSilentFailure = errors.New("")

type rootOptions struct {
	quiet   bool
	noColor bool
	profile string
}

var rootOpts rootOptions

var rootCmd = &cobra.Command{
	Use:   "PreFlight",
	Short: "PreFlight is a CLI tool for checking project dependencies.",
	Long: `A CLI tool that validates your project dependencies before you run into problems. Checks if everything is installed, fixes what's missing, and runs security audits across package managers.

Supports npm, yarn, pnpm, bun, Composer, Go, pip, Poetry, uv, and Bundler.
Configure with preflight.yml for profiles, scripts, and CI integration.`,
	Example:       "preflight check --pm=npm,composer",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		terminal.SetQuiet(rootOpts.quiet)

		if rootOpts.noColor {
			terminal.DisableColor()
		}

		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = false

	rootCmd.PersistentFlags().BoolVar(&rootOpts.quiet, "quiet", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&rootOpts.noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVar(&rootOpts.profile, "profile", "", "Active profile in preflight.yml (overrides PREFLIGHT_PROFILE)")
}
