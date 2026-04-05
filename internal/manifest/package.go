package manifest

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type PackageJSON struct {
	Engines struct {
		Node string `json:"node"`
		NPM  string `json:"npm,omitempty"`
		PNPM string `json:"pnpm,omitempty"`
		Yarn string `json:"yarn,omitempty"`
		Bun  string `json:"bun,omitempty"`
	} `json:"engines"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type PackageConfig struct {
	PackageManager  PackageManager
	NodeVersion     string
	NPMVersion      string
	PNPMVersion     string
	YarnVersion     string
	BunVersion      string
	Dependencies    []string
	DevDependencies []string
	HasConfig       bool
	Error           error
}

func (l Loader) LoadPackageConfig() PackageConfig {
	config := PackageConfig{}
	config.PackageManager, _ = l.DetectPackageManager(PackageTypeJS)
	config.HasConfig = config.PackageManager.ConfigFileExists

	if !config.HasConfig {
		return config
	}

	raw, err := l.readFile("package.json")

	if err != nil {
		config.Error = fmt.Errorf("failed to read package.json: %w", err)
		return config
	}

	var data PackageJSON

	if err := json.Unmarshal(raw, &data); err != nil {
		config.Error = fmt.Errorf("failed to parse package.json: %w", err)
		return config
	}

	parsePackageJSON(&config, &data)

	return config
}

func parsePackageJSON(config *PackageConfig, data *PackageJSON) {
	config.NodeVersion = strings.TrimSpace(data.Engines.Node)
	config.NPMVersion = strings.TrimSpace(data.Engines.NPM)
	config.PNPMVersion = strings.TrimSpace(data.Engines.PNPM)
	config.YarnVersion = strings.TrimSpace(data.Engines.Yarn)
	config.BunVersion = strings.TrimSpace(data.Engines.Bun)

	config.Dependencies = slices.Sorted(maps.Keys(data.Dependencies))
	config.DevDependencies = slices.Sorted(maps.Keys(data.DevDependencies))
}
