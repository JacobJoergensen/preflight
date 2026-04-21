package monorepo

import (
	"os"
	"path/filepath"
	"sort"
)

func DiscoverProjects(workDir string) ([]Project, error) {
	seen := make(map[string]struct{})

	var projects []Project

	sources := []func(string) ([]Project, error){
		discoverPnpmWorkspace,
		discoverPackageJSONWorkspaces,
		discoverGoWork,
	}

	for _, source := range sources {
		discovered, err := source(workDir)

		if err != nil {
			return nil, err
		}

		for _, project := range discovered {
			if _, dup := seen[project.AbsolutePath]; dup {
				continue
			}

			seen[project.AbsolutePath] = struct{}{}
			projects = append(projects, project)
		}
	}

	if len(projects) == 0 {
		walked, err := discoverByWalk(workDir)

		if err != nil {
			return nil, err
		}

		projects = walked
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].RelativePath < projects[j].RelativePath
	})

	return projects, nil
}

func FilterByGlobs(projects []Project, patterns []string) ([]Project, error) {
	if len(patterns) == 0 {
		return projects, nil
	}

	filtered := make([]Project, 0, len(projects))

	for _, project := range projects {
		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, project.RelativePath)

			if err != nil {
				return nil, err
			}

			if matched {
				filtered = append(filtered, project)
				break
			}
		}
	}

	return filtered, nil
}

func projectsFromGlobs(workDir string, patterns []string) ([]Project, error) {
	directories, err := expandGlobPatterns(workDir, patterns)

	if err != nil {
		return nil, err
	}

	projects := make([]Project, 0, len(directories))

	for _, dir := range directories {
		project, err := projectFromDir(workDir, dir)

		if err != nil {
			return nil, err
		}

		projects = append(projects, project)
	}

	return projects, nil
}

func expandGlobPatterns(workDir string, patterns []string) ([]string, error) {
	var directories []string

	for _, pattern := range patterns {
		absPattern := filepath.Join(workDir, filepath.FromSlash(pattern))

		matches, err := filepath.Glob(absPattern)

		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			info, statErr := os.Stat(match)

			if statErr != nil || !info.IsDir() {
				continue
			}

			directories = append(directories, match)
		}
	}

	return directories, nil
}

func projectFromDir(workDir, absDir string) (Project, error) {
	rel, err := filepath.Rel(workDir, absDir)

	if err != nil {
		return Project{}, err
	}

	return Project{
		RelativePath: filepath.ToSlash(rel),
		AbsolutePath: absDir,
		Name:         readProjectName(absDir),
	}, nil
}

func readProjectName(absDir string) string {
	if name := readNpmName(absDir); name != "" {
		return name
	}

	if name := readGoModuleName(absDir); name != "" {
		return name
	}

	return ""
}
