package engine

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
)

type CheckProgress interface {
	Plan(total int)
	Start(scopeID, displayName string)
	Finish(scopeID string, included bool)
	Close()
}

type NoopCheckProgress struct{}

func (NoopCheckProgress) Plan(int)             {}
func (NoopCheckProgress) Start(string, string) {}
func (NoopCheckProgress) Finish(string, bool)  {}
func (NoopCheckProgress) Close()               {}

func (r Runner) Check(
	ctx context.Context,
	scopes []string,
	selectors []string,
	withEnv bool,
	outdated bool,
	progress CheckProgress,
	disableMonorepo bool,
	projectGlobs []string,
) (result.CheckReport, error) {
	if progress == nil {
		progress = NoopCheckProgress{}
	}

	if !disableMonorepo {
		projects, err := monorepo.DiscoverProjects(r.WorkDir)

		if err != nil {
			return result.CheckReport{}, fmt.Errorf("monorepo discovery failed: %w", err)
		}

		projects, err = monorepo.FilterByGlobs(projects, projectGlobs)

		if err != nil {
			return result.CheckReport{}, fmt.Errorf("project filter failed: %w", err)
		}

		if len(projects) > 0 {
			return r.checkMonorepo(ctx, projects, scopes, selectors, withEnv, outdated, progress)
		}
	}

	return r.checkProject(ctx, r.WorkDir, "", scopes, selectors, withEnv, outdated, progress)
}

func (r Runner) checkMonorepo(
	ctx context.Context,
	projects []monorepo.Project,
	scopes []string,
	selectors []string,
	withEnv bool,
	outdated bool,
	progress CheckProgress,
) (result.CheckReport, error) {
	startedAt := time.Now()

	var allItems []result.CheckItem

	projectSummaries := make([]result.CheckProject, 0, len(projects))

	for _, project := range projects {
		projectSummaries = append(projectSummaries, result.CheckProject{
			RelativePath: project.RelativePath,
			Name:         project.Name,
		})

		projectReport, err := r.checkProject(ctx, project.AbsolutePath, project.RelativePath, scopes, selectors, withEnv, outdated, progress)

		if err != nil {
			return result.CheckReport{}, err
		}

		allItems = append(allItems, projectReport.Items...)
	}

	return result.CheckReport{
		StartedAt: startedAt,
		EndedAt:   time.Now(),
		Canceled:  ctx.Err() != nil,
		Items:     allItems,
		Projects:  projectSummaries,
	}, nil
}

func (r Runner) checkProject(
	ctx context.Context,
	workDir string,
	projectPath string,
	scopes []string,
	selectors []string,
	withEnv bool,
	outdated bool,
	progress CheckProgress,
) (result.CheckReport, error) {
	selection, err := Select(SelectInput{Scopes: scopes, Selectors: selectors, Mode: ModeCheck})

	if err != nil {
		return result.CheckReport{}, err
	}

	deps := r.depsForDir(workDir)

	if err := validateRequestedPackageManagers(selectors, deps); err != nil {
		return result.CheckReport{}, err
	}

	adapters := filterComposerUnlessExplicit(selection.Adapters, deps, scopes, selectors)

	if isImplicitFullSelection(scopes, selectors) {
		adapters = withoutAdapter(adapters, "env")
	}

	adapters = appendEnvIfRequested(adapters, withEnv, scopes, selectors)

	report := runChecks(ctx, adapters, deps, progress)

	if outdated {
		attachOutdatedPackages(ctx, adapters, deps, report.Items)
	}

	if projectPath != "" {
		for i := range report.Items {
			report.Items[i].Project = projectPath
		}
	}

	return report, nil
}

func runChecks(ctx context.Context, modules []adapter.Adapter, deps adapter.Dependencies, progress CheckProgress) result.CheckReport {
	startedAt := time.Now()

	progress.Plan(len(modules))

	items := runParallel(ctx, modules, func(ctx context.Context, m adapter.Adapter) (result.CheckItem, bool) {
		scopeID := m.Name()

		progress.Start(scopeID, adapter.DisplayName(m))

		var included bool
		defer func() { progress.Finish(scopeID, included) }()

		itemStartedAt := time.Now()
		errors, warnings, successes := m.Check(ctx, deps)
		itemEndedAt := time.Now()

		if len(errors) == 0 && len(warnings) == 0 && len(successes) == 0 {
			return result.CheckItem{}, false
		}

		included = true

		return result.CheckItem{
			ScopeID:        m.Name(),
			ScopeDisplay:   adapter.DisplayName(m),
			Priority:       adapter.GetPriority(m.Name()),
			Errors:         errors,
			Warnings:       warnings,
			Successes:      successes,
			StartedAt:      itemStartedAt,
			EndedAt:        itemEndedAt,
			ElapsedMillis:  itemEndedAt.Sub(itemStartedAt).Milliseconds(),
			ProjectSignals: adapter.ProjectSignals(m.Name(), deps.Loader),
			FixPMHint:      adapter.FixPMHint(m.Name(), deps.Loader),
		}, true
	})

	slices.SortFunc(items, func(a, b result.CheckItem) int {
		return cmp.Compare(a.Priority, b.Priority)
	})

	return result.CheckReport{
		StartedAt: startedAt,
		EndedAt:   time.Now(),
		Canceled:  ctx.Err() != nil,
		Items:     items,
	}
}

func attachOutdatedPackages(ctx context.Context, adapters []adapter.Adapter, deps adapter.Dependencies, items []result.CheckItem) {
	itemByScopeID := make(map[string]int, len(items))

	for i, item := range items {
		itemByScopeID[item.ScopeID] = i
	}

	for _, a := range adapters {
		outdatedLister, ok := a.(adapter.OutdatedLister)

		if !ok {
			continue
		}

		packages, err := outdatedLister.ListOutdated(ctx, deps)

		if err != nil || len(packages) == 0 {
			continue
		}

		if idx, ok := itemByScopeID[a.Name()]; ok {
			items[idx].Outdated = packages
		}
	}
}
