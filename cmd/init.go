package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func newInitCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create preflight.yml with detected scopes and profiles",
		Long: `Creates a starter preflight.yml in the current directory using manifests
present in the project (composer.json, package.json, go.mod, etc.).

Existing files are left untouched unless --force is set.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			workDir, err := workingDir()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			path := filepath.Join(workDir, config.FileName)

			_, statErr := os.Stat(path)

			if statErr == nil && !force {
				return errors.New("preflight.yml already exists (use --force to overwrite)")
			}

			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return fmt.Errorf("stat preflight.yml: %w", statErr)
			}

			body, err := config.Generate(workDir, fs.OSFS{})
			if err != nil {
				return err
			}

			if err := os.WriteFile(path, body, 0o600); err != nil {
				return fmt.Errorf("write preflight.yml: %w", err)
			}

			if !terminal.Quiet {
				if _, err := fmt.Fprintf(os.Stdout, "Wrote %s\n", path); err != nil {
					return fmt.Errorf("write stdout: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing preflight.yml")

	return cmd
}
