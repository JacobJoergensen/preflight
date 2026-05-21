package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/manifest"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
)

type Runner struct {
	WorkDir       string
	FS            fs.FS
	Command       exec.Runner
	CommandStream exec.StreamRunner
}

func NewRunner(workDir string) Runner {
	return Runner{
		WorkDir:       workDir,
		FS:            fs.OSFS{},
		Command:       exec.DefaultRunner{},
		CommandStream: exec.DefaultStreamRunner{},
	}
}

func discoverProjects(workDir string, disableMonorepo bool, projectGlobs []string) ([]monorepo.Project, error) {
	if disableMonorepo {
		return nil, nil
	}

	projects, err := monorepo.DiscoverProjects(workDir)
	if err != nil {
		return nil, fmt.Errorf("monorepo discovery failed: %w", err)
	}

	projects, err = monorepo.FilterByGlobs(projects, projectGlobs)
	if err != nil {
		return nil, fmt.Errorf("project filter failed: %w", err)
	}

	return projects, nil
}

func aggregateProjects[T any](projects []monorepo.Project, perProject func(monorepo.Project) ([]T, error)) ([]T, []result.Project, error) {
	var items []T

	summaries := make([]result.Project, 0, len(projects))

	for _, project := range projects {
		summaries = append(summaries, result.Project{
			RelativePath: project.RelativePath,
			Name:         project.Name,
		})

		projectItems, err := perProject(project)
		if err != nil {
			return nil, nil, err
		}

		items = append(items, projectItems...)
	}

	return items, summaries, nil
}

func (r Runner) depsForDir(workDir string) adapter.Dependencies {
	loader := manifest.NewLoader(workDir)
	loader.FS = r.FS

	return adapter.Dependencies{
		Loader: loader,
		FS:     r.FS,
		Runner: r.Command,
		Stream: r.CommandStream,
	}
}

func appendEnvIfRequested(adapters []adapter.Adapter, withEnv bool, only []string) []adapter.Adapter {
	if !withEnv || selectionIncludesEnv(only) {
		return adapters
	}

	envAdapters, err := adapter.Select("env")

	if err != nil || len(envAdapters) == 0 {
		return adapters
	}

	return append(adapters, envAdapters[0])
}

func selectionIncludesEnv(only []string) bool {
	for _, s := range only {
		if strings.EqualFold(strings.TrimSpace(s), "env") {
			return true
		}
	}

	return false
}

func filterComposerUnlessExplicit(adapters []adapter.Adapter, deps adapter.Dependencies, only []string) []adapter.Adapter {
	if !isImplicitFullSelection(only) {
		return adapters
	}

	if deps.Loader.HasComposerJSON() {
		return adapters
	}

	return withoutAdapter(adapters, "composer")
}

func isImplicitFullSelection(only []string) bool {
	return len(nonEmptyStrings(only)) == 0
}

func nonEmptyStrings(ss []string) []string {
	output := make([]string, 0, len(ss))

	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			output = append(output, s)
		}
	}

	return output
}

func withoutAdapter(adapters []adapter.Adapter, name string) []adapter.Adapter {
	return slices.DeleteFunc(slices.Clone(adapters), func(a adapter.Adapter) bool {
		return a.Name() == name
	})
}

func validateRequestedPackageManagers(only []string, deps adapter.Dependencies) error {
	for _, selector := range only {
		selector = strings.ToLower(strings.TrimSpace(selector))

		if selector == "" {
			continue
		}

		// Skip if it's a scope (js, python, etc.) rather than a specific PM
		if manifest.IsPackageType(selector) {
			continue
		}

		packageType, isPackageManager := manifest.GetPackageType(selector)

		if !isPackageManager {
			continue
		}

		// Only validate for package types with multiple managers (js, python)
		if packageType != manifest.PackageTypeJS && packageType != manifest.PackageTypePython {
			continue
		}

		detected, ok := deps.Loader.DetectPackageManager(packageType)

		if !ok {
			return fmt.Errorf("requested %s but no %s project detected", selector, packageType)
		}

		if !strings.EqualFold(detected.Command(), selector) {
			return fmt.Errorf("requested %s but project uses %s", selector, detected.Command())
		}
	}

	return nil
}
