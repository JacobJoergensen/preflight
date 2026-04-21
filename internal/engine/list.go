package engine

import (
	"context"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
)

func (r Runner) List(ctx context.Context, scopes []string, selectors []string, outdated bool) (result.DependencyReport, error) {
	selection, err := Select(SelectInput{Scopes: scopes, Selectors: selectors, Mode: ModeList})

	if err != nil {
		return result.DependencyReport{}, err
	}

	deps := r.deps()

	if err := validateRequestedPackageManagers(selectors, deps); err != nil {
		return result.DependencyReport{}, err
	}

	adapters := filterComposerUnlessExplicit(selection.Adapters, deps, scopes, selectors)
	startedAt := time.Now()
	depsByAdapter := make(map[string][]string)
	elapsedByAdapter := make(map[string]int64)
	displaysByAdapter := make(map[string]string)

	var outdatedByAdapter map[string][]adapter.OutdatedPackage

	if outdated {
		outdatedByAdapter = make(map[string][]adapter.OutdatedPackage)
	}

	for _, a := range adapters {
		lister, ok := a.(adapter.DependencyLister)

		if !ok {
			continue
		}

		adapterStartedAt := time.Now()

		list, listErr := lister.ListDependencies(ctx, deps)

		if listErr != nil || len(list) == 0 {
			continue
		}

		depsByAdapter[a.Name()] = list
		displaysByAdapter[a.Name()] = adapter.DisplayName(a)

		if outdated {
			if outdatedLister, ok := a.(adapter.OutdatedLister); ok {
				packages, err := outdatedLister.ListOutdated(ctx, deps)

				if err == nil && len(packages) > 0 {
					outdatedByAdapter[a.Name()] = packages
				}
			}
		}

		elapsedByAdapter[a.Name()] = time.Since(adapterStartedAt).Milliseconds()
	}

	return result.DependencyReport{
		StartedAt:    startedAt,
		EndedAt:      time.Now(),
		AdapterIDs:   adapter.Names(adapters),
		Displays:     displaysByAdapter,
		Dependencies: depsByAdapter,
		Outdated:     outdatedByAdapter,
		Elapsed:      elapsedByAdapter,
	}, nil
}
