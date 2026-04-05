package cmd

import (
	"os"

	"github.com/JacobJoergensen/preflight/internal/config"
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
