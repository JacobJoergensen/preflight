package engine

import (
	"cmp"
	"context"
	"strings"
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
)

func (r Runner) Audit(
	ctx context.Context,
	only []string,
	minSeverity string,
	ignoredCVEs []string,
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
		return r.auditMonorepo(ctx, projects, only, minSeverity, ignoredCVEs, progress)
	}

	return r.auditProject(ctx, r.WorkDir, "", only, minSeverity, ignoredCVEs, progress)
}

func (r Runner) auditMonorepo(
	ctx context.Context,
	projects []monorepo.Project,
	only []string,
	minSeverity string,
	ignoredCVEs []string,
	progress ScanProgress,
) (result.AuditReport, error) {
	startedAt := time.Now()

	allItems, projectSummaries, err := aggregateProjects(projects, func(project monorepo.Project) ([]result.AuditItem, error) {
		projectReport, err := r.auditProject(ctx, project.AbsolutePath, project.RelativePath, only, minSeverity, ignoredCVEs, progress)
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
	ignoredCVEs []string,
	progress ScanProgress,
) (result.AuditReport, error) {
	selection, err := Select(SelectInput{Only: only, Mode: ModeAudit})
	if err != nil {
		return result.AuditReport{}, err
	}

	rc := r.runContextForDir(workDir)

	if err := validateRequestedPackageManagers(only, rc); err != nil {
		return result.AuditReport{}, err
	}

	specs := filterComposerUnlessExplicit(selection.Specs, rc, only)

	if isImplicitFullSelection(only) {
		specs = withoutSpec(specs, "env")
	}

	report := runAudits(ctx, specs, rc, progress)

	report = filterAuditReportByIgnored(report, ignoredCVEs)

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

func runAudits(ctx context.Context, specs []*ecosystem.Spec, rc ecosystem.RunContext, progress ScanProgress) result.AuditReport {
	run := runScopes(ctx, specs, rc, progress,
		func(ctx context.Context, spec *ecosystem.Spec, detection ecosystem.Detection) (result.AuditItem, bool) {
			startedAt := time.Now()
			auditResult := spec.RunAudit(ctx, rc, detection)
			endedAt := time.Now()

			if auditResult.Skipped {
				return result.AuditItem{}, false
			}

			return result.FromAuditResult(spec.Name, spec.Title(), spec.Priority, auditResult, startedAt, endedAt), true
		},
		func(left, right result.AuditItem) int {
			if diff := cmp.Compare(right.SeverityRank, left.SeverityRank); diff != 0 {
				return diff
			}

			if diff := cmp.Compare(left.Priority, right.Priority); diff != 0 {
				return diff
			}

			return cmp.Compare(left.ScopeID, right.ScopeID)
		},
	)

	return result.AuditReport{
		StartedAt: run.StartedAt,
		EndedAt:   run.EndedAt,
		Canceled:  run.Canceled,
		Items:     run.Items,
	}
}

func filterAuditReportByIgnored(report result.AuditReport, ignored []string) result.AuditReport {
	suppress := make(map[string]struct{}, len(ignored))

	for _, id := range ignored {
		if normalized := normalizeAdvisoryID(id); normalized != "" {
			suppress[normalized] = struct{}{}
		}
	}

	if len(suppress) == 0 {
		return report
	}

	for i := range report.Items {
		item := &report.Items[i]

		// Only items that actually produced findings are eligible: an item with no
		// findings (a parse gap or a tool error) must keep its original status.
		if item.ErrText != "" || len(item.Findings) == 0 {
			continue
		}

		kept := make([]model.Finding, 0, len(item.Findings))

		for _, finding := range item.Findings {
			if !findingSuppressed(finding, suppress) {
				kept = append(kept, finding)
			}
		}

		if len(kept) == len(item.Findings) {
			continue
		}

		item.Findings = kept
		item.SeverityRank = ecosystem.SeverityRankFromFindings(kept)

		// Every finding was an accepted advisory, so the audit passes.
		if len(kept) == 0 {
			item.OK = true
		}
	}

	return report
}

func findingSuppressed(finding model.Finding, suppress map[string]struct{}) bool {
	if _, ok := suppress[normalizeAdvisoryID(finding.ID)]; ok {
		return true
	}

	for _, alias := range finding.Aliases {
		if _, ok := suppress[normalizeAdvisoryID(alias)]; ok {
			return true
		}
	}

	return false
}

func normalizeAdvisoryID(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}

func filterAuditReportBySeverity(report result.AuditReport, minSeverity string) result.AuditReport {
	threshold := ecosystem.SeverityLevel(minSeverity)

	for i := range report.Items {
		item := &report.Items[i]

		if item.ErrText != "" {
			continue
		}

		item.Findings = filterFindingsBySeverity(item.Findings, threshold)
		item.SeverityRank = ecosystem.SeverityRankFromFindings(item.Findings)
		item.OK = len(item.Findings) == 0
	}

	return report
}

func filterFindingsBySeverity(findings []model.Finding, threshold int) []model.Finding {
	if len(findings) == 0 {
		return findings
	}

	filtered := make([]model.Finding, 0, len(findings))

	for _, finding := range findings {
		if ecosystem.SeverityLevel(finding.Severity) >= threshold {
			filtered = append(filtered, finding)
		}
	}

	return filtered
}
