package lockdiff

import (
	"encoding/json"
	"fmt"
	"strings"
)

const nodeModulesPrefix = "node_modules/"

type npmParser struct{}

func (npmParser) Ecosystem() string { return "node" }

func (npmParser) Parse(data []byte) (map[string]string, error) {
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}

	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse package-lock.json: %w", err)
	}

	packages := make(map[string]string)

	for path, entry := range lock.Packages {
		if path == "" || entry.Version == "" {
			continue
		}

		name := packageNameFromPath(path)

		if name == "" {
			continue
		}

		packages[name] = entry.Version
	}

	if len(packages) == 0 {
		for name, entry := range lock.Dependencies {
			if entry.Version == "" {
				continue
			}

			packages[name] = entry.Version
		}
	}

	return packages, nil
}

func packageNameFromPath(path string) string {
	idx := strings.LastIndex(path, nodeModulesPrefix)

	if idx < 0 {
		return ""
	}

	return path[idx+len(nodeModulesPrefix):]
}

func init() {
	Register("package-lock.json", npmParser{})
}
