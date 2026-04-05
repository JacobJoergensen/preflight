package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/JacobJoergensen/preflight/internal/fs"
)

func Load(workDir string, filesystem fs.FS) (File, error) {
	path := filepath.Join(workDir, FileName)
	raw, err := filesystem.ReadFile(path)

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, nil
		}

		return File{}, fmt.Errorf("read preflight.yml: %w", err)
	}

	var config File

	if err := yaml.Unmarshal(raw, &config); err != nil {
		return File{}, fmt.Errorf("parse preflight.yml: %w", err)
	}

	if config.Version == 0 {
		return File{}, fmt.Errorf("preflight.yml: missing or invalid version (expected %d)", SchemaVersion)
	}

	if err := config.validate(); err != nil {
		return File{}, err
	}

	return config, nil
}
