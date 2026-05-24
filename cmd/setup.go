package cmd

import (
	"fmt"
	"os"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/fs"
)

// workingDir is the directory PreFlight operates in: the --cwd flag when set,
// otherwise the process working directory.
func workingDir() (string, error) {
	if rootOpts.cwd != "" {
		return rootOpts.cwd, nil
	}

	return os.Getwd()
}

func loadPreflightConfig(workDir string) (config.File, string, error) {
	cfg, err := config.Load(workDir, fs.OSFS{})
	if err != nil {
		return config.File{}, "", err
	}

	name := config.ResolveProfileName(rootOpts.profile, os.Getenv("PREFLIGHT_PROFILE"), cfg.Profile)

	return cfg, name, nil
}

func commandSetup(failMessage string) (engine.Runner, config.Profile, error) {
	workDir, err := workingDir()
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
