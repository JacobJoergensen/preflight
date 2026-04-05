package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/hooks"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type hooksInstallOptions struct {
	force   bool
	command string
}

var hooksInstallOpts hooksInstallOptions

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage Git hooks that run PreFlight",
	Long: `Install or remove a marked block in .git/hooks/pre-commit so PreFlight can coexist
with Husky, Lefthook, or other hand-written hooks. The block is delimited by
# BEGIN PREFLIGHT and # END PREFLIGHT comments.`,
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Add a PreFlight block to the Git pre-commit hook",
	Long: `Creates or updates .git/hooks/pre-commit with a shell block that runs PreFlight.

If pre-commit already exists and has no PreFlight markers, the command fails unless
--force is set (append block at the end).`,
	Example: `preflight hooks install
preflight hooks install --force
preflight hooks install --command "preflight check --with-env"`,
	RunE: func(_ *cobra.Command, _ []string) error {
		workDir, err := os.Getwd()

		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		path, err := hooks.PreCommitPath(workDir)

		if err != nil {
			return err
		}

		command := hooksInstallOpts.command

		err = hooks.Install(path, command, hooksInstallOpts.force)

		if err != nil {
			if errors.Is(err, hooks.ErrHookExists) {
				return fmt.Errorf("%s%w%s", terminal.Red, err, terminal.Reset)
			}

			return err
		}

		if !terminal.Quiet {
			if _, writeErr := fmt.Fprintf(os.Stdout, "Updated %s\n", path); writeErr != nil {
				return fmt.Errorf("write stdout: %w", writeErr)
			}
		}

		return nil
	},
}

var hooksRemoveCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"uninstall"},
	Short:   "Remove the PreFlight block from the Git pre-commit hook",
	Long: `Removes only the marked PreFlight section. Other hook content is left intact.
If the file becomes empty, it is deleted.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		workDir, err := os.Getwd()

		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		path, err := hooks.PreCommitPath(workDir)

		if err != nil {
			return err
		}

		if err := hooks.Remove(path); err != nil {
			return err
		}

		if !terminal.Quiet {
			if _, writeErr := fmt.Fprintf(os.Stdout, "Removed PreFlight block from %s\n", path); writeErr != nil {
				return fmt.Errorf("write stdout: %w", writeErr)
			}
		}

		return nil
	},
}

func init() {
	hooksInstallCmd.Flags().BoolVar(&hooksInstallOpts.force, "force", false,
		"Append PreFlight block when pre-commit exists without # BEGIN PREFLIGHT markers")
	hooksInstallCmd.Flags().StringVar(&hooksInstallOpts.command, "command", hooks.DefaultHookCommand,
		"Shell command to run inside the hook (default: preflight check)")

	hooksCmd.AddCommand(hooksInstallCmd, hooksRemoveCmd)

	rootCmd.AddCommand(hooksCmd)
}
