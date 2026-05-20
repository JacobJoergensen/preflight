package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/release"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

var ErrSilentFailure = errors.New("")

const (
	exitSuccess  = 0
	exitFindings = 1
	exitError    = 2
)

type rootOptions struct {
	quiet   bool
	noColor bool
	profile string
	version bool
}

var rootOpts rootOptions

var rootCmd = &cobra.Command{
	Use:   "preflight",
	Short: "PreFlight is a CLI tool for checking project dependencies.",
	Long: `A CLI tool that validates your project dependencies before you run into problems. Checks if everything is installed, fixes what's missing, and runs security audits across package managers.

Supports npm, yarn, pnpm, bun, Composer, Go, pip, Poetry, uv, and Bundler.
Configure with preflight.yml for profiles, scripts, and CI integration.`,
	Example:       "preflight check --only npm,composer",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		terminal.SetQuiet(rootOpts.quiet)
		terminal.ConfigureColor(rootOpts.noColor, os.Stdout)

		return nil
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		if rootOpts.version {
			printVersion(terminal.NewOutputWriter())
			return nil
		}

		return cmd.Help()
	},
}

func Execute() int {
	err := rootCmd.Execute()

	if err != nil && !errors.Is(err, ErrSilentFailure) {
		_, _ = fmt.Fprintf(os.Stderr, "%s%s%s\n", terminal.Red, err, terminal.Reset)
	}

	return exitCode(err)
}

func exitCode(err error) int {
	switch {
	case err == nil:
		return exitSuccess
	case errors.Is(err, ErrSilentFailure):
		return exitFindings
	default:
		return exitError
	}
}

func printVersion(out *terminal.OutputWriter) {
	version, commit, date := release.BuildInfo()

	if version == "" {
		version = "dev"
	}

	if terminal.Quiet {
		out.Println(version)
		return
	}

	out.Printf("%spreflight%s %s\n", terminal.Bold, terminal.Reset, version)

	if commit != "" {
		out.Printf("  %scommit%s    %s\n", terminal.Dim, terminal.Reset, commit)
	}

	if built := formatBuildDate(date); built != "" {
		out.Printf("  %sbuilt%s     %s\n", terminal.Dim, terminal.Reset, built)
	}

	out.Printf("  %splatform%s  %s/%s\n", terminal.Dim, terminal.Reset, runtime.GOOS, runtime.GOARCH)
}

func formatBuildDate(raw string) string {
	if raw == "" {
		return ""
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}

	return parsed.UTC().Format("2006-01-02 15:04 UTC")
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = false

	rootCmd.Flags().BoolVarP(&rootOpts.version, "version", "v", false, "Print version, commit, build date, and platform")

	rootCmd.PersistentFlags().BoolVar(&rootOpts.quiet, "quiet", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&rootOpts.noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVar(&rootOpts.profile, "profile", "", "Active profile in preflight.yml (overrides PREFLIGHT_PROFILE)")
}
