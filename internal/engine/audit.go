package engine

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
)

func (r Runner) Audit(
	ctx context.Context,
	only []string,
	minSeverity string,
	progress ScanProgress,
	disableMonorepo bool,
	projectGlobs []string,
) (result.AuditReport, error) {
	if progress == nil {
		progress = NoopScanProgress{}
	}

	projects, err := discoverProjects(r.WorkDir, disableMonorepo, projectGlobs)
	if err != nil {
		return result.AuditReport{}, err
	}

	if len(projects) > 0 {
		return r.auditMonorepo(ctx, projects, only, minSeverity, progress)
	}

	return r.auditProject(ctx, r.WorkDir, "", only, minSeverity, progress)
}

func (r Runner) auditMonorepo(
	ctx context.Context,
	projects []monorepo.Project,
	only []string,
	minSeverity string,
	progress ScanProgress,
) (result.AuditReport, error) {
	startedAt := time.Now()

	allItems, projectSummaries, err := aggregateProjects(projects, func(project monorepo.Project) ([]result.AuditItem, error) {
		projectReport, err := r.auditProject(ctx, project.AbsolutePath, project.RelativePath, only, minSeverity, progress)
		return projectReport.Items, err
	})
	if err != nil {
		return result.AuditReport{}, err
	}

	return result.AuditReport{
		StartedAt: startedAt,
		EndedAt:   time.Now(),
		Canceled:  ctx.Err() != nil,
		Items:     allItems,
		Projects:  projectSummaries,
	}, nil
}

func (r Runner) auditProject(
	ctx context.Context,
	workDir string,
	projectPath string,
	only []string,
	minSeverity string,
	progress ScanProgress,
) (result.AuditReport, error) {
	selection, err := Select(SelectInput{Only: only, Mode: ModeAudit})
	if err != nil {
		return result.AuditReport{}, err
	}

	deps := r.depsForDir(workDir)

	if err := validateRequestedPackageManagers(only, deps); err != nil {
		return result.AuditReport{}, err
	}

	adapters := filterComposerUnlessExplicit(selection.Adapters, deps, only)

	if isImplicitFullSelection(only) {
		adapters = withoutAdapter(adapters, "env")
	}

	runners := filterAuditRunners(adapters)
	report := runAudits(ctx, runners, deps, progress)

	if minSeverity != "" {
		report = filterAuditReportBySeverity(report, minSeverity)
	}

	if projectPath != "" {
		for i := range report.Items {
			report.Items[i].Project = projectPath
		}
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

func runAudits(ctx context.Context, runners []adapter.AuditRunner, deps adapter.Dependencies, progress ScanProgress) result.AuditReport {
	startedAt := time.Now()

	progress.Plan(len(runners))

	items := runParallel(ctx, runners, func(ctx context.Context, runner adapter.AuditRunner) (result.AuditItem, bool) {
		scopeID := runner.Name()

		progress.Start(scopeID, adapter.DisplayName(runner))

		var included bool
		defer func() { progress.Finish(scopeID, included) }()

		itemStartedAt := time.Now()
		auditResult := runner.Audit(ctx, deps)
		itemEndedAt := time.Now()

		if auditResult.Skipped {
			return result.AuditItem{}, false
		}

		included = true

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
		if item.ErrText != "" {
			filtered = append(filtered, item)
			continue
		}

		filteredCounts := filterCountsBySeverity(item.Counts, threshold)
		hasIssues := len(filteredCounts) > 0

		filtered = append(filtered, result.AuditItem{
			Project:       item.Project,
			ScopeID:       item.ScopeID,
			ScopeDisplay:  item.ScopeDisplay,
			Priority:      item.Priority,
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
