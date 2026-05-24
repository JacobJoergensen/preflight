package monorepo

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

func discoverUvWorkspace(workDir string) ([]Project, error) {
	raw, err := readManifest(workDir, "pyproject.toml")
	if err != nil || raw == nil {
		return nil, err
	}

	var doc struct {
		Tool struct {
			UV struct {
				Workspace struct {
					Members []string `toml:"members"`
				} `toml:"workspace"`
			} `toml:"uv"`
		} `toml:"tool"`
	}

	if err := toml.Unmarshal(raw, &doc); err != nil || len(doc.Tool.UV.Workspace.Members) == 0 {
		return nil, nil
	}

	return projectsFromGlobs(workDir, doc.Tool.UV.Workspace.Members)
}

func readPyName(absDir string) string {
	// #nosec G304 - absDir is a discovered subproject directory; the fixed "pyproject.toml" suffix means we only read declared manifests.
	raw, err := os.ReadFile(filepath.Join(absDir, "pyproject.toml"))
	if err != nil {
		return ""
	}

	var doc struct {
		Project struct {
			Name string `toml:"name"`
		} `toml:"project"`
	}

	if err := toml.Unmarshal(raw, &doc); err != nil {
		return ""
	}

	return doc.Project.Name
}
