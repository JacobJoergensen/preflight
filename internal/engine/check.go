package engine

import (
	"cmp"
	"context"
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
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
	run := runScopes(ctx, specs, rc, progress,
		func(ctx context.Context, spec *ecosystem.Spec, detection ecosystem.Detection) (result.CheckItem, bool) {
			startedAt := time.Now()
			messages := spec.RunCheck(ctx, rc, detection)
			endedAt := time.Now()

			if len(messages) == 0 {
				return result.CheckItem{}, false
			}

			return result.CheckItem{
				ScopeID:        spec.Name,
				ScopeDisplay:   spec.Title(),
				Priority:       spec.Priority,
				Messages:       messages,
				StartedAt:      startedAt,
				EndedAt:        endedAt,
				ElapsedMillis:  endedAt.Sub(startedAt).Milliseconds(),
				ProjectSignals: spec.Signals(rc, detection),
				FixPMHint:      spec.FixPMHint(detection),
			}, true
		},
		func(a, b result.CheckItem) int {
			return cmp.Compare(a.Priority, b.Priority)
		},
	)

	return result.CheckReport{
		StartedAt: run.StartedAt,
		EndedAt:   run.EndedAt,
		Canceled:  run.Canceled,
		Items:     run.Items,
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
