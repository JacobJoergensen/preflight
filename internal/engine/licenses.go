package engine

import (
	"cmp"
	"context"
	"strings"
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
)

func (r Runner) Licenses(
	ctx context.Context,
	only []string,
	allow []string,
	deny []string,
	progress ScanProgress,
	disableMonorepo bool,
	projectGlobs []string,
) (result.LicenseReport, error) {
	if progress == nil {
		progress = NoopScanProgress{}
	}

	projects, err := discoverProjects(r.WorkDir, disableMonorepo, projectGlobs)
	if err != nil {
		return result.LicenseReport{}, err
	}

	policy := newLicensePolicy(allow, deny)

	if len(projects) > 0 {
		return r.licensesMonorepo(ctx, projects, only, policy, progress)
	}

	return r.licensesProject(ctx, r.WorkDir, "", only, policy, progress)
}

func (r Runner) licensesMonorepo(
	ctx context.Context,
	projects []monorepo.Project,
	only []string,
	policy licensePolicy,
	progress ScanProgress,
) (result.LicenseReport, error) {
	startedAt := time.Now()

	allItems, projectSummaries, err := aggregateProjects(projects, func(project monorepo.Project) ([]result.LicenseItem, error) {
		projectReport, err := r.licensesProject(ctx, project.AbsolutePath, project.RelativePath, only, policy, progress)
		return projectReport.Items, err
	})
	if err != nil {
		return result.LicenseReport{}, err
	}

	return result.LicenseReport{
		StartedAt: startedAt,
		EndedAt:   time.Now(),
		Canceled:  ctx.Err() != nil,
		Items:     allItems,
		Projects:  projectSummaries,
	}, nil
}

func (r Runner) licensesProject(
	ctx context.Context,
	workDir string,
	projectPath string,
	only []string,
	policy licensePolicy,
	progress ScanProgress,
) (result.LicenseReport, error) {
	selection, err := Select(SelectInput{Only: only, Mode: ModeAudit})
	if err != nil {
		return result.LicenseReport{}, err
	}

	rc := r.runContextForDir(workDir)

	if err := validateRequestedPackageManagers(only, rc); err != nil {
		return result.LicenseReport{}, err
	}

	report := runLicenses(ctx, selection.Specs, rc, policy, progress)

	if projectPath != "" {
		for i := range report.Items {
			report.Items[i].Project = projectPath
		}
	}

	return report, nil
}

func runLicenses(ctx context.Context, specs []*ecosystem.Spec, rc ecosystem.RunContext, policy licensePolicy, progress ScanProgress) result.LicenseReport {
	run := runScopes(ctx, specs, rc, progress,
		func(ctx context.Context, spec *ecosystem.Spec, detection ecosystem.Detection) (result.LicenseItem, bool) {
			if spec.License == nil {
				return result.LicenseItem{}, false
			}

			startedAt := time.Now()
			licenseResult := spec.License(ctx, rc, detection)
			endedAt := time.Now()

			if licenseResult.Skipped {
				return result.LicenseItem{}, false
			}

			violations := evaluateLicenses(licenseResult.Packages, policy)

			return result.FromLicenseResult(spec.Name, spec.Title(), spec.Priority, licenseResult, violations, startedAt, endedAt), true
		},
		func(left, right result.LicenseItem) int {
			if diff := cmp.Compare(len(right.Violations), len(left.Violations)); diff != 0 {
				return diff
			}

			if diff := cmp.Compare(left.Priority, right.Priority); diff != 0 {
				return diff
			}

			return cmp.Compare(left.ScopeID, right.ScopeID)
		},
	)

	return result.LicenseReport{
		StartedAt: run.StartedAt,
		EndedAt:   run.EndedAt,
		Canceled:  run.Canceled,
		Items:     run.Items,
	}
}

type licensePolicy struct {
	allow map[string]struct{}
	deny  map[string]struct{}
}

func newLicensePolicy(allow, deny []string) licensePolicy {
	return licensePolicy{allow: licenseSet(allow), deny: licenseSet(deny)}
}

func licenseSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))

	for _, id := range ids {
		if normalized := strings.ToLower(strings.TrimSpace(id)); normalized != "" {
			set[normalized] = struct{}{}
		}
	}

	return set
}

func evaluateLicenses(packages []ecosystem.PackageLicense, policy licensePolicy) []result.LicenseViolation {
	var violations []result.LicenseViolation

	for _, pkg := range packages {
		reason, violated := policy.check(pkg.License)
		if !violated {
			continue
		}

		violations = append(violations, result.LicenseViolation{
			Package: pkg.Name,
			Version: pkg.Version,
			License: pkg.License,
			Reason:  reason,
		})
	}

	return violations
}

func (p licensePolicy) check(license string) (string, bool) {
	tokens := tokenizeLicense(license)

	if len(tokens) == 0 {
		return "", false
	}

	for _, token := range tokens {
		if _, denied := p.deny[token]; denied {
			return "license " + token + " is denied", true
		}
	}

	if len(p.allow) == 0 {
		return "", false
	}

	for _, token := range tokens {
		if _, allowed := p.allow[token]; allowed {
			return "", false
		}
	}

	return "license " + strings.TrimSpace(license) + " is not in the allowlist", true
}

func tokenizeLicense(license string) []string {
	replacer := strings.NewReplacer("(", " ", ")", " ", "/", " ", ",", " ")
	fields := strings.Fields(replacer.Replace(strings.ToLower(license)))

	tokens := make([]string, 0, len(fields))

	for _, field := range fields {
		switch field {
		case "or", "and", "with":
			continue
		default:
			tokens = append(tokens, field)
		}
	}

	return tokens
}
