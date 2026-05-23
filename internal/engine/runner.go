package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs"
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

func (r Runner) runContextForDir(workDir string) ecosystem.RunContext {
	return ecosystem.RunContext{
		WorkDir: workDir,
		FS:      r.FS,
		Runner:  r.Command,
		Stream:  r.CommandStream,
	}
}

func appendEnvIfRequested(specs []*ecosystem.Spec, withEnv bool, only []string) []*ecosystem.Spec {
	if !withEnv || selectionIncludesEnv(only) {
		return specs
	}

	envSpec, ok := ecosystem.Lookup("env")

	if !ok {
		return specs
	}

	return append(specs, envSpec)
}

func selectionIncludesEnv(only []string) bool {
	for _, s := range only {
		if strings.EqualFold(strings.TrimSpace(s), "env") {
			return true
		}
	}

	return false
}

func filterComposerUnlessExplicit(specs []*ecosystem.Spec, rc ecosystem.RunContext, only []string) []*ecosystem.Spec {
	if !isImplicitFullSelection(only) {
		return specs
	}

	if rc.FileExists("composer.json") {
		return specs
	}

	return withoutSpec(specs, "composer")
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

func withoutSpec(specs []*ecosystem.Spec, name string) []*ecosystem.Spec {
	return slices.DeleteFunc(slices.Clone(specs), func(spec *ecosystem.Spec) bool {
		return spec.Name == name
	})
}

func validateRequestedPackageManagers(only []string, rc ecosystem.RunContext) error {
	for _, selector := range only {
		selector = strings.ToLower(strings.TrimSpace(selector))

		if selector == "" || ecosystem.IsScope(selector) {
			continue
		}

		scope, ok := ecosystem.ScopeForManager(selector)

		if !ok {
			continue
		}

		spec, ok := ecosystem.Lookup(scope)

		if !ok || len(spec.Managers) < 2 {
			continue
		}

		detection, ok := spec.Resolve(rc)

		if !ok {
			return fmt.Errorf("requested %s but no %s project detected", selector, scope)
		}

		if !strings.EqualFold(detection.Active.Command, selector) {
			return fmt.Errorf("requested %s but project uses %s", selector, detection.Active.Command)
		}
	}

	return nil
}
