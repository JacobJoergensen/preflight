package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/fs"
)

func loadPreflightConfig(workDir string) (config.File, string, error) {
	cfg, err := config.Load(workDir, fs.OSFS{})
	if err != nil {
		return config.File{}, "", err
	}

	name := config.ResolveProfileName(rootOpts.profile, os.Getenv("PREFLIGHT_PROFILE"), cfg.Profile)

	return cfg, name, nil
}

func commandSetup(failMessage string) (engine.Runner, config.Profile, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return engine.Runner{}, config.Profile{}, fmt.Errorf("get working directory: %w", err)
	}

	cfg, profileName, err := loadPreflightConfig(workDir)
	if err != nil {
		return engine.Runner{}, config.Profile{}, fmt.Errorf("%s: %w", failMessage, err)
	}

	profile, err := cfg.ProfileFor(profileName)
	if err != nil {
		return engine.Runner{}, config.Profile{}, err
	}

	return engine.NewRunner(workDir), profile, nil
}

func resolveOnly(cmd *cobra.Command, cliOnly []string, profileOnly *[]string) []string {
	if cmd.Flags().Changed("only") {
		return cliOnly
	}

	if profileOnly != nil {
		return *profileOnly
	}

	return cliOnly
}
