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

type initOptions struct {
	force bool
}

var initOpts initOptions

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create preflight.yml with detected scopes and profiles",
	Long: `Creates a starter preflight.yml in the current directory using manifests
present in the project (composer.json, package.json, go.mod, etc.).

Existing files are left untouched unless --force is set.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		workDir, err := os.Getwd()

		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		path := filepath.Join(workDir, config.FileName)

		_, statErr := os.Stat(path)

		if statErr == nil && !initOpts.force {
			return fmt.Errorf("%spreflight.yml already exists (use --force to overwrite)%s", terminal.Red, terminal.Reset)
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

func init() {
	initCmd.Flags().BoolVar(&initOpts.force, "force", false, "Overwrite an existing preflight.yml")

	rootCmd.AddCommand(initCmd)
}
