package lockdiff

import (
	"encoding/json"
	"fmt"
)

type composerParser struct{}

func (composerParser) Ecosystem() string { return "composer" }

func (composerParser) Parse(data []byte) (map[string]string, error) {
	var lock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
		PackagesDev []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages-dev"`
	}

	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse composer.lock: %w", err)
	}

	packages := make(map[string]string, len(lock.Packages)+len(lock.PackagesDev))

	for _, pkg := range lock.Packages {
		if pkg.Name == "" {
			continue
		}

		packages[pkg.Name] = pkg.Version
	}

	for _, pkg := range lock.PackagesDev {
		if pkg.Name == "" {
			continue
		}

		packages[pkg.Name] = pkg.Version
	}

	return packages, nil
}

func init() {
	Register("composer.lock", composerParser{})
}
