package engine

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
	"github.com/JacobJoergensen/preflight/internal/parallel"
)

func (r Runner) Check(
	ctx context.Context,
	only []string,
	withEnv bool,
	outdated bool,
	progress ScanProgress,
	disableMonorepo bool,
	projectGlobs []string,
) (result.CheckReport, error) {
	if progress == nil {
		progress = NoopScanProgress{}
	}

	projects, err := discoverProjects(r.WorkDir, disableMonorepo, projectGlobs)
	if err != nil {
		return result.CheckReport{}, err
	}

	if len(projects) > 0 {
		return r.checkMonorepo(ctx, projects, only, withEnv, outdated, progress)
	}

	return r.checkProject(ctx, r.WorkDir, "", only, withEnv, outdated, progress)
}

func (r Runner) checkMonorepo(
	ctx context.Context,
	projects []monorepo.Project,
	only []string,
	withEnv bool,
	outdated bool,
	progress ScanProgress,
) (result.CheckReport, error) {
	startedAt := time.Now()

	allItems, projectSummaries, err := aggregateProjects(projects, func(project monorepo.Project) ([]result.CheckItem, error) {
		projectReport, err := r.checkProject(ctx, project.AbsolutePath, project.RelativePath, only, withEnv, outdated, progress)
		return projectReport.Items, err
	})
	if err != nil {
		return result.CheckReport{}, err
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
	only []string,
	withEnv bool,
	outdated bool,
	progress ScanProgress,
) (result.CheckReport, error) {
	selection, err := Select(SelectInput{Only: only, Mode: ModeCheck})
	if err != nil {
		return result.CheckReport{}, err
	}

	rc := r.runContextForDir(workDir)

	if err := validateRequestedPackageManagers(only, rc); err != nil {
		return result.CheckReport{}, err
	}

	specs := filterComposerUnlessExplicit(selection.Specs, rc, only)

	if isImplicitFullSelection(only) {
		specs = withoutSpec(specs, "env")
	}

	specs = appendEnvIfRequested(specs, withEnv, only)

	report := runChecks(ctx, specs, rc, progress)

	if outdated {
		attachOutdatedPackages(ctx, specs, rc, report.Items)
	}

	if projectPath != "" {
		for i := range report.Items {
			report.Items[i].Project = projectPath
		}
	}

	return report, nil
}

func runChecks(ctx context.Context, specs []*ecosystem.Spec, rc ecosystem.RunContext, progress ScanProgress) result.CheckReport {
	startedAt := time.Now()

	progress.Plan(len(specs))

	items := parallel.Collect(ctx, specs, func(ctx context.Context, spec *ecosystem.Spec) (result.CheckItem, bool) {
		scopeID := spec.Name

		progress.Start(scopeID, spec.Title())

		var included bool
		defer func() { progress.Finish(scopeID, included) }()

		detection, ok := spec.Resolve(rc)

		if !ok {
			return result.CheckItem{}, false
		}

		itemStartedAt := time.Now()
		messages := spec.RunCheck(ctx, rc, detection)
		itemEndedAt := time.Now()

		if len(messages) == 0 {
			return result.CheckItem{}, false
		}

		included = true

		return result.CheckItem{
			ScopeID:        spec.Name,
			ScopeDisplay:   spec.Title(),
			Priority:       spec.Priority,
			Messages:       messages,
			StartedAt:      itemStartedAt,
			EndedAt:        itemEndedAt,
			ElapsedMillis:  itemEndedAt.Sub(itemStartedAt).Milliseconds(),
			ProjectSignals: spec.Signals(rc, detection),
			FixPMHint:      spec.FixPMHint(detection),
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

func attachOutdatedPackages(ctx context.Context, specs []*ecosystem.Spec, rc ecosystem.RunContext, items []result.CheckItem) {
	itemByScopeID := make(map[string]int, len(items))

	for i, item := range items {
		itemByScopeID[item.ScopeID] = i
	}

	for _, spec := range specs {
		if !spec.CanOutdated() {
			continue
		}

		detection, ok := spec.Resolve(rc)

		if !ok {
			continue
		}

		packages, err := spec.RunOutdated(ctx, rc, detection)

		if err != nil || len(packages) == 0 {
			continue
		}

		if idx, ok := itemByScopeID[spec.Name]; ok {
			items[idx].Outdated = packages
		}
	}
}
