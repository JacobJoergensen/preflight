package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
)

func (r Runner) List(
	ctx context.Context,
	scopes []string,
	selectors []string,
	outdated bool,
	disableMonorepo bool,
	projectGlobs []string,
) (result.DependencyReport, error) {
	if !disableMonorepo {
		projects, err := monorepo.DiscoverProjects(r.WorkDir)

		if err != nil {
			return result.DependencyReport{}, fmt.Errorf("monorepo discovery failed: %w", err)
		}

		projects, err = monorepo.FilterByGlobs(projects, projectGlobs)

		if err != nil {
			return result.DependencyReport{}, fmt.Errorf("project filter failed: %w", err)
		}

		if len(projects) > 0 {
			return r.listMonorepo(ctx, projects, scopes, selectors, outdated)
		}
	}

	return r.listProject(ctx, r.WorkDir, "", scopes, selectors, outdated)
}

func (r Runner) listMonorepo(
	ctx context.Context,
	projects []monorepo.Project,
	scopes []string,
	selectors []string,
	outdated bool,
) (result.DependencyReport, error) {
	startedAt := time.Now()

	var allItems []result.DependencyItem

	projectSummaries := make([]result.DependencyProject, 0, len(projects))

	for _, project := range projects {
		projectSummaries = append(projectSummaries, result.DependencyProject{
			RelativePath: project.RelativePath,
			Name:         project.Name,
		})

		projectReport, err := r.listProject(ctx, project.AbsolutePath, project.RelativePath, scopes, selectors, outdated)

		if err != nil {
			return result.DependencyReport{}, err
		}

		allItems = append(allItems, projectReport.Items...)
	}

	return result.DependencyReport{
		StartedAt: startedAt,
		EndedAt:   time.Now(),
		Canceled:  ctx.Err() != nil,
		Items:     allItems,
		Projects:  projectSummaries,
	}, nil
}

func (r Runner) listProject(
	ctx context.Context,
	workDir string,
	projectPath string,
	scopes []string,
	selectors []string,
	outdated bool,
) (result.DependencyReport, error) {
	selection, err := Select(SelectInput{Scopes: scopes, Selectors: selectors, Mode: ModeList})

	if err != nil {
		return result.DependencyReport{}, err
	}

	deps := r.depsForDir(workDir)

	if err := validateRequestedPackageManagers(selectors, deps); err != nil {
		return result.DependencyReport{}, err
	}

	adapters := filterComposerUnlessExplicit(selection.Adapters, deps, scopes, selectors)
	startedAt := time.Now()

	items := make([]result.DependencyItem, 0, len(adapters))

	for _, a := range adapters {
		if ctx.Err() != nil {
			break
		}

		lister, ok := a.(adapter.DependencyLister)

		if !ok {
			continue
		}

		adapterStartedAt := time.Now()

		dependencies, listErr := lister.ListDependencies(ctx, deps)

		if listErr != nil || len(dependencies) == 0 {
			continue
		}

		item := result.DependencyItem{
			Project:       projectPath,
			AdapterID:     a.Name(),
			Display:       adapter.DisplayName(a),
			Dependencies:  dependencies,
			ElapsedMillis: time.Since(adapterStartedAt).Milliseconds(),
		}

		if outdated {
			if outdatedLister, ok := a.(adapter.OutdatedLister); ok {
				packages, err := outdatedLister.ListOutdated(ctx, deps)

				if err == nil && len(packages) > 0 {
					item.Outdated = packages
				}
			}
		}

		items = append(items, item)
	}

	return result.DependencyReport{
		StartedAt: startedAt,
		EndedAt:   time.Now(),
		Canceled:  ctx.Err() != nil,
		Items:     items,
	}, nil
}
