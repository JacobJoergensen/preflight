package monorepo

import (
	"os"
	"path/filepath"
	"strings"
)

const walkMaxDepth = 4

var walkSkipDirectories = map[string]struct{}{
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"build":        {},
	"target":       {},
	"tmp":          {},
	"out":          {},
	"coverage":     {},
	"__pycache__":  {},
	"venv":         {},
}

var walkManifestFiles = []string{
	"package.json",
	"composer.json",
	"go.mod",
	"pyproject.toml",
	"Gemfile",
	"gems.rb",
}

func discoverByWalk(workDir string) ([]Project, error) {
	var found []Project

	if err := walkCollectProjects(workDir, workDir, 0, &found); err != nil {
		return nil, err
	}

	if len(found) < 2 {
		return nil, nil
	}

	return found, nil
}

func walkCollectProjects(rootDir, currentDir string, depth int, found *[]Project) error {
	if depth > walkMaxDepth {
		return nil
	}

	if hasProjectManifest(currentDir) {
		project, err := projectFromDir(rootDir, currentDir)

		if err != nil {
			return err
		}

		*found = append(*found, project)
	}

	entries, err := os.ReadDir(currentDir)

	if err != nil {
		// Unreadable directories are silently skipped so a single permissions error doesn't abort discovery for the whole tree.
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		if shouldSkipWalkDirectory(name) {
			continue
		}

		if err := walkCollectProjects(rootDir, filepath.Join(currentDir, name), depth+1, found); err != nil {
			return err
		}
	}

	return nil
}

func hasProjectManifest(dir string) bool {
	for _, manifest := range walkManifestFiles {
		if _, err := os.Stat(filepath.Join(dir, manifest)); err == nil {
			return true
		}
	}

	return false
}

func shouldSkipWalkDirectory(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}

	_, skip := walkSkipDirectories[name]
	return skip
}
