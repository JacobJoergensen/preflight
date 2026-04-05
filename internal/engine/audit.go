package engine

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
)

func (r Runner) Audit(ctx context.Context, scopes []string, selectors []string) (result.AuditReport, error) {
	selection, err := Select(SelectInput{Scopes: scopes, Selectors: selectors, Mode: ModeAudit})

	if err != nil {
		return result.AuditReport{}, err
	}

	deps := r.deps()

	if err := validateRequestedPackageManagers(selectors, deps); err != nil {
		return result.AuditReport{}, err
	}

	adapters := filterComposerUnlessExplicit(selection.Adapters, deps, scopes, selectors)

	if isImplicitFullSelection(scopes, selectors) {
		adapters = withoutAdapter(adapters, "env")
	}

	runners := filterAuditRunners(adapters)

	return runAudits(ctx, runners, deps), nil
}

func filterAuditRunners(adapters []adapter.Adapter) []adapter.AuditRunner {
	runners := make([]adapter.AuditRunner, 0, len(adapters))

	for _, adp := range adapters {
		if runner, ok := adp.(adapter.AuditRunner); ok {
			runners = append(runners, runner)
		}
	}

	return runners
}

func runAudits(ctx context.Context, runners []adapter.AuditRunner, deps adapter.Dependencies) result.AuditReport {
	startedAt := time.Now()

	items := runParallel(ctx, runners, func(ctx context.Context, runner adapter.AuditRunner) (result.AuditItem, bool) {
		itemStartedAt := time.Now()
		auditResult := runner.Audit(ctx, deps)
		itemEndedAt := time.Now()

		return result.FromAdapterAudit(
			runner.Name(),
			adapter.DisplayName(runner),
			adapter.GetPriority(runner.Name()),
			auditResult,
			itemStartedAt,
			itemEndedAt,
		), true
	})

	slices.SortFunc(items, func(left, right result.AuditItem) int {
		if diff := cmp.Compare(right.SeverityRank, left.SeverityRank); diff != 0 {
			return diff
		}

		if diff := cmp.Compare(left.Priority, right.Priority); diff != 0 {
			return diff
		}

		return cmp.Compare(left.ScopeID, right.ScopeID)
	})

	return result.AuditReport{
		StartedAt: startedAt,
		EndedAt:   time.Now(),
		Canceled:  ctx.Err() != nil,
		Items:     items,
	}
}
