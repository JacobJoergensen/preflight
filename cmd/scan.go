package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/render"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type scanFlags struct {
	only         []string
	format       string
	timeout      time.Duration
	noMonorepo   bool
	projectGlobs []string
}

func registerScanFlags(cmd *cobra.Command, f *scanFlags, timeoutDefault time.Duration, verb string) {
	cmd.Flags().StringSliceVar(&f.only, "only", nil, "Limit to these ecosystems or tools (comma-separated: js, npm, composer, go, rust, python, ruby, env)")
	cmd.Flags().StringVarP(&f.format, "format", "o", "text", "Output format: text or json")
	cmd.Flags().DurationVarP(&f.timeout, "timeout", "t", timeoutDefault, "Timeout for "+verb)
	cmd.Flags().BoolVar(&f.noMonorepo, "no-monorepo", false, "Disable monorepo traversal, "+verb+" only the current directory")
	cmd.Flags().StringSliceVar(&f.projectGlobs, "project", nil, "Restrict monorepo traversal to projects matching these path globs (comma-separated, e.g. packages/*)")
}

func parseFormat(format string) (jsonOutput bool, err error) {
	switch format {
	case "", "text":
		return false, nil
	case "json":
		return true, nil
	default:
		return false, fmt.Errorf("invalid --format %q (use text or json)", format)
	}
}

type scanCommand[R any] struct {
	failMsg  string
	timeout  time.Duration
	run      func(ctx context.Context, runner engine.Runner, profile config.Profile) (R, error)
	render   func(R) error
	markdown func(R, io.Writer) error
	exitCode func(R) int
	// afterRender runs after the report is rendered, before the exit code is
	// applied. When it reports handled, the command exits successfully. Optional.
	afterRender func(R) (handled bool, err error)
}

func runScan[R any](cmd *cobra.Command, sc scanCommand[R]) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), sc.timeout)
	defer cancel()

	runner, profile, err := commandSetup(sc.failMsg)
	if err != nil {
		return err
	}

	report, err := sc.run(ctx, runner, profile)
	if err != nil {
		return fmt.Errorf("%s: %w", sc.failMsg, err)
	}

	if err := sc.render(report); err != nil {
		return err
	}

	writeGitHubSummary(func(w io.Writer) error {
		return sc.markdown(report, w)
	})

	if sc.afterRender != nil {
		handled, err := sc.afterRender(report)
		if err != nil {
			return fmt.Errorf("%s: %w", sc.failMsg, err)
		}

		if handled {
			return nil
		}
	}

	if sc.exitCode(report) != exitSuccess {
		return ErrSilentFailure
	}

	return nil
}

func flagOrProfile[T any](cmd *cobra.Command, flagName string, cliVal T, profileVal *T) T {
	if cmd.Flags().Changed(flagName) || profileVal == nil {
		return cliVal
	}

	return *profileVal
}

func reportExitCode[T any](canceled bool, items []T, failed func(T) bool) int {
	if canceled {
		return exitFindings
	}

	if slices.ContainsFunc(items, failed) {
		return exitFindings
	}

	return exitSuccess
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
