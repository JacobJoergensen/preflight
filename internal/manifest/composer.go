package manifest

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type ComposerJSON struct {
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

type ComposerConfig struct {
	PackageManager  PackageManager
	PHPVersion      string
	PHPExtensions   []string
	Dependencies    []string
	DevDependencies []string
	HasConfig       bool
	HasLock         bool
	Error           error
}

func (l Loader) HasComposerJSON() bool {
	return l.FileExists("composer.json")
}

func (l Loader) HasComposerPHPContext() bool {
	return l.HasComposerJSON() || l.FileExists("composer.lock")
}

func (l Loader) LoadComposerConfig() ComposerConfig {
	config := ComposerConfig{}
	config.PackageManager, _ = l.DetectPackageManager("composer")
	config.HasConfig = config.PackageManager.ConfigFileExists
	config.HasLock = config.PackageManager.LockFileExists

	if !config.HasConfig {
		return config
	}

	raw, err := l.readFile("composer.json")

	if err != nil {
		config.Error = fmt.Errorf("failed to read composer.json: %w", err)
		return config
	}

	var data ComposerJSON

	if err := json.Unmarshal(raw, &data); err != nil {
		config.Error = fmt.Errorf("failed to parse composer.json: %w", err)
		return config
	}

	parseComposerJSON(&config, &data)

	return config
}

func parseComposerJSON(config *ComposerConfig, data *ComposerJSON) {
	var deps, exts []string

	for dep, version := range data.Require {
		switch {
		case dep == "php":
			config.PHPVersion = version
		case strings.HasPrefix(dep, "ext-"):
			exts = append(exts, strings.TrimPrefix(dep, "ext-"))
		default:
			deps = append(deps, dep)
		}
	}

	slices.Sort(deps)
	slices.Sort(exts)

	config.Dependencies = deps
	config.PHPExtensions = exts

	var devDeps []string

	for dep := range data.RequireDev {
		if !strings.HasPrefix(dep, "ext-") {
			devDeps = append(devDeps, dep)
		}
	}

	slices.Sort(devDeps)
	config.DevDependencies = devDeps
}
