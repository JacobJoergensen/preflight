package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/hooks"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func newHooksCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage Git hooks that run PreFlight",
		Long: `Install or remove a marked block in the repository's pre-commit hook (honoring
core.hooksPath) so PreFlight can coexist with Husky, Lefthook, or other hand-written
hooks. The block is delimited by # BEGIN PREFLIGHT and # END PREFLIGHT comments.`,
	}

	cmd.AddCommand(newHooksInstallCommand(), newHooksRemoveCommand())

	return cmd
}

func newHooksInstallCommand() *cobra.Command {
	var (
		force   bool
		command string
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Add a PreFlight block to the Git pre-commit hook",
		Long: `Creates or updates the repository's pre-commit hook with a shell block that runs
PreFlight, writing to the directory configured by core.hooksPath when it is set.

If pre-commit already exists and has no PreFlight markers, the command fails unless
--force is set (append block at the end).`,
		Example: `preflight hooks install
preflight hooks install --force
preflight hooks install --command "preflight check --with-env"`,
		RunE: func(_ *cobra.Command, _ []string) error {
			workDir, err := workingDir()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			path, err := hooks.PreCommitPath(workDir)
			if err != nil {
				return err
			}

			err = hooks.Install(path, command, force)

			if errors.Is(err, hooks.ErrHookExists) && !terminal.Quiet && terminal.IsInteractive() {
				confirmed, askErr := terminal.Ask(os.Stdin, os.Stdout, "A pre-commit hook already exists. Append PreFlight to it?")
				if askErr != nil {
					return askErr
				}

				if !confirmed {
					if _, writeErr := fmt.Fprintln(os.Stdout, "Left the pre-commit hook unchanged."); writeErr != nil {
						return fmt.Errorf("write stdout: %w", writeErr)
					}

					return nil
				}

				err = hooks.Install(path, command, true)
			}

			if err != nil {
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

	cmd.Flags().BoolVar(&force, "force", false,
		"Append PreFlight block when pre-commit exists without # BEGIN PREFLIGHT markers")
	cmd.Flags().StringVar(&command, "command", hooks.DefaultHookCommand,
		"Shell command to run inside the hook (default: preflight check)")

	return cmd
}

func newHooksRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove",
		Aliases: []string{"uninstall"},
		Short:   "Remove the PreFlight block from the Git pre-commit hook",
		Long: `Removes only the marked PreFlight section. Other hook content is left intact.
If the file becomes empty, it is deleted.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			workDir, err := workingDir()
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
}
