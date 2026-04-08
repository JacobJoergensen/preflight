package engine

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
)

func (r Runner) Audit(ctx context.Context, scopes []string, selectors []string, minSeverity string) (result.AuditReport, error) {
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
	report := runAudits(ctx, runners, deps)

	if minSeverity != "" {
		report = filterAuditReportBySeverity(report, minSeverity)
	}

	return report, nil
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

func filterAuditReportBySeverity(report result.AuditReport, minSeverity string) result.AuditReport {
	threshold := adapter.SeverityLevel(minSeverity)
	filtered := make([]result.AuditItem, 0, len(report.Items))

	for _, item := range report.Items {
		if item.Skipped || item.ErrText != "" {
			filtered = append(filtered, item)
			continue
		}

		filteredCounts := filterCountsBySeverity(item.Counts, threshold)
		hasIssues := len(filteredCounts) > 0

		filtered = append(filtered, result.AuditItem{
			ScopeID:       item.ScopeID,
			ScopeDisplay:  item.ScopeDisplay,
			Priority:      item.Priority,
			Skipped:       item.Skipped,
			SkipReason:    item.SkipReason,
			CommandLine:   item.CommandLine,
			ExitCode:      item.ExitCode,
			OK:            !hasIssues,
			SeverityRank:  recalculateSeverityRank(filteredCounts),
			Counts:        filteredCounts,
			Output:        item.Output,
			ErrText:       item.ErrText,
			StartedAt:     item.StartedAt,
			EndedAt:       item.EndedAt,
			ElapsedMillis: item.ElapsedMillis,
		})
	}

	report.Items = filtered

	return report
}

func filterCountsBySeverity(counts map[string]int, threshold int) map[string]int {
	if len(counts) == 0 {
		return counts
	}

	filtered := make(map[string]int)

	for name, count := range counts {
		if count <= 0 {
			continue
		}

		if adapter.SeverityLevel(name) >= threshold {
			filtered[name] = count
		}
	}

	return filtered
}

func recalculateSeverityRank(counts map[string]int) int {
	if len(counts) == 0 {
		return 0
	}

	rank := 0

	for name, count := range counts {
		if count <= 0 {
			continue
		}

		switch adapter.SeverityLevel(name) {
		case 4:
			rank += 1000 * count
		case 3:
			rank += 100 * count
		case 2:
			rank += 10 * count
		case 1:
			rank += count
		}
	}

	return rank
}
