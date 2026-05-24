package monorepo

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

func discoverPnpmWorkspace(workDir string) ([]Project, error) {
	raw, err := readManifest(workDir, "pnpm-workspace.yaml")
	if err != nil || raw == nil {
		return nil, err
	}

	var config struct {
		Packages []string `yaml:"packages"`
	}

	if err := yaml.Unmarshal(raw, &config); err != nil {
		return nil, err
	}

	return projectsFromGlobs(workDir, config.Packages)
}

func discoverPackageJSONWorkspaces(workDir string) ([]Project, error) {
	raw, err := readManifest(workDir, "package.json")
	if err != nil || raw == nil {
		return nil, err
	}

	var probe struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}

	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, nil
	}

	if len(probe.Workspaces) == 0 {
		return nil, nil
	}

	patterns := workspacePatternsFromJSON(probe.Workspaces)

	if len(patterns) == 0 {
		return nil, nil
	}

	return projectsFromGlobs(workDir, patterns)
}

func workspacePatternsFromJSON(raw json.RawMessage) []string {
	var arrayForm []string

	if err := json.Unmarshal(raw, &arrayForm); err == nil {
		return arrayForm
	}

	var objectForm struct {
		Packages []string `json:"packages"`
	}

	if err := json.Unmarshal(raw, &objectForm); err == nil {
		return objectForm.Packages
	}

	return nil
}

func readNpmName(absDir string) string {
	// #nosec G304 - absDir is a discovered subproject directory resolved during workspace traversal; the fixed "package.json" suffix means we only read declared manifest files.
	raw, err := os.ReadFile(filepath.Join(absDir, "package.json"))
	if err != nil {
		return ""
	}

	var probe struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}

	return probe.Name
}
