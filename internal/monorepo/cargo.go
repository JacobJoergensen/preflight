package monorepo

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

func discoverCargoWorkspace(workDir string) ([]Project, error) {
	raw, err := readManifest(workDir, "Cargo.toml")
	if err != nil || raw == nil {
		return nil, err
	}

	var doc struct {
		Workspace struct {
			Members []string `toml:"members"`
		} `toml:"workspace"`
	}

	if err := toml.Unmarshal(raw, &doc); err != nil || len(doc.Workspace.Members) == 0 {
		return nil, nil
	}

	return projectsFromGlobs(workDir, doc.Workspace.Members)
}

func readCargoName(absDir string) string {
	// #nosec G304 - absDir is a discovered subproject directory; the fixed "Cargo.toml" suffix means we only read declared manifests.
	raw, err := os.ReadFile(filepath.Join(absDir, "Cargo.toml"))
	if err != nil {
		return ""
	}

	var doc struct {
		Package struct {
			Name string `toml:"name"`
		} `toml:"package"`
	}

	if err := toml.Unmarshal(raw, &doc); err != nil {
		return ""
	}

	return doc.Package.Name
}
