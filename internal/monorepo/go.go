package monorepo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func discoverGoWork(workDir string) ([]Project, error) {
	path := filepath.Join(workDir, "go.work")

	// #nosec G304 - path is workDir joined with the fixed "go.work" filename; workDir is supplied by the caller (cmd layer resolves it from os.Getwd), not user input at read time.
	raw, err := os.ReadFile(path)

	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	useDirectories := parseGoWorkUseDirectives(string(raw))

	projects := make([]Project, 0, len(useDirectories))

	for _, dir := range useDirectories {
		absDir := filepath.Join(workDir, filepath.FromSlash(dir))

		info, statErr := os.Stat(absDir)

		if statErr != nil || !info.IsDir() {
			continue
		}

		project, err := projectFromDir(workDir, absDir)

		if err != nil {
			return nil, err
		}

		projects = append(projects, project)
	}

	return projects, nil
}

func parseGoWorkUseDirectives(content string) []string {
	var directories []string

	inUseBlock := false

	for line := range strings.SplitSeq(content, "\n") {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}

		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if inUseBlock {
			if line == ")" {
				inUseBlock = false
				continue
			}

			directories = append(directories, strings.Trim(line, `"`))

			continue
		}

		if line == "use (" {
			inUseBlock = true
			continue
		}

		if strings.HasPrefix(line, "use ") {
			dir := strings.TrimSpace(strings.TrimPrefix(line, "use"))
			directories = append(directories, strings.Trim(dir, `"`))
		}
	}

	return directories
}

func readGoModuleName(absDir string) string {
	// #nosec G304 - absDir is a discovered sub-project directory resolved during workspace traversal; the fixed "go.mod" suffix means we only read declared module manifests, not arbitrary user input.
	raw, err := os.ReadFile(filepath.Join(absDir, "go.mod"))

	if err != nil {
		return ""
	}

	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "module ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "module"))
			return strings.Trim(name, `"`)
		}
	}

	return ""
}
