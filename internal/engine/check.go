package engine

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
)

func (r Runner) Check(ctx context.Context, scopes []string, selectors []string, withEnv bool, outdated bool) (result.CheckReport, error) {
	selection, err := Select(SelectInput{Scopes: scopes, Selectors: selectors, Mode: ModeCheck})

	if err != nil {
		return result.CheckReport{}, err
	}

	deps := r.deps()

	if err := validateRequestedPackageManagers(selectors, deps); err != nil {
		return result.CheckReport{}, err
	}

	adapters := filterComposerUnlessExplicit(selection.Adapters, deps, scopes, selectors)

	// Env adapter is opt-in, strip from implicit selection and re-add only if requested
	if isImplicitFullSelection(scopes, selectors) {
		adapters = withoutAdapter(adapters, "env")
	}

	adapters = appendEnvIfRequested(adapters, withEnv, scopes, selectors)

	report := runChecks(ctx, adapters, deps)

	if outdated {
		report.Outdated = make(map[string][]adapter.OutdatedPackage)

		for _, a := range adapters {
			if outdatedLister, ok := a.(adapter.OutdatedLister); ok {
				packages, err := outdatedLister.ListOutdated(ctx, deps)

				if err == nil && len(packages) > 0 {
					report.Outdated[a.Name()] = packages
				}
			}
		}
	}

	return report, nil
}

func runChecks(ctx context.Context, modules []adapter.Adapter, deps adapter.Dependencies) result.CheckReport {
	startedAt := time.Now()

	items := runParallel(ctx, modules, func(ctx context.Context, m adapter.Adapter) (result.CheckItem, bool) {
		itemStartedAt := time.Now()
		errors, warnings, successes := m.Check(ctx, deps)
		itemEndedAt := time.Now()

		if len(errors) == 0 && len(warnings) == 0 && len(successes) == 0 {
			return result.CheckItem{}, false
		}

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
