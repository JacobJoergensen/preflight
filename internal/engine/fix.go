package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
)

type FixCandidate struct {
	Project     string
	ScopeID     string
	DisplayName string
	Command     string
	Summary     string
}

type FixDecision int

const (
	FixApply FixDecision = iota
	FixSkip
	FixAbort
)

type FixApprover interface {
	Approve(candidate FixCandidate) (FixDecision, error)
}

type AutoFixApprover struct{}

type FixProgress interface {
	Plan(candidates []FixCandidate)
	Start(candidate FixCandidate)
	Finish(item result.FixItem)
}

type NoopFixProgress struct{}

func (AutoFixApprover) Approve(FixCandidate) (FixDecision, error) {
	return FixApply, nil
}

func (NoopFixProgress) Plan([]FixCandidate)   {}
func (NoopFixProgress) Start(FixCandidate)    {}
func (NoopFixProgress) Finish(result.FixItem) {}

func (r Runner) Fix(
	ctx context.Context,
	only []string,
	opts ecosystem.FixOptions,
	diff bool,
	approver FixApprover,
	progress FixProgress,
	disableMonorepo bool,
	projectGlobs []string,
) (result.FixReport, error) {
	if approver == nil {
		approver = AutoFixApprover{}
	}

	if progress == nil {
		progress = NoopFixProgress{}
	}

	projects, err := discoverProjects(r.WorkDir, disableMonorepo, projectGlobs)
	if err != nil {
		return result.FixReport{}, err
	}

	if len(projects) > 0 {
		return r.fixMonorepo(ctx, projects, only, opts, diff, approver, progress)
	}

	return r.fixSingleProject(ctx, r.WorkDir, "", only, opts, diff, approver, progress)
}

type projectFixPrep struct {
	project           monorepo.Project
	rc                ecosystem.RunContext
	specs             []*ecosystem.Spec
	selection         Selection
	candidates        []FixCandidate
	prepError         error
	skippedOnPrepFail result.SkippedFix
}

func (r Runner) fixMonorepo(
	ctx context.Context,
	projects []monorepo.Project,
	only []string,
	opts ecosystem.FixOptions,
	diff bool,
	approver FixApprover,
	progress FixProgress,
) (result.FixReport, error) {
	startedAt := time.Now()

	projectSummaries := make([]result.Project, 0, len(projects))

	for _, project := range projects {
		projectSummaries = append(projectSummaries, result.Project{
			RelativePath: project.RelativePath,
			Name:         project.Name,
		})
	}

	preps, fatalErr := r.prepareMonorepoProjects(projects, only)

	if fatalErr != nil {
		return result.FixReport{}, fatalErr
	}

	var allCandidates []FixCandidate

	for _, prep := range preps {
		allCandidates = append(allCandidates, prep.candidates...)
	}

	plannedFixes := plannedFixesFromCandidates(allCandidates)
	plan, aborted, err := resolveFixPlan(allCandidates, approver)
	if err != nil {
		return result.FixReport{}, fmt.Errorf("approval failed: %w", err)
	}

	skipped := collectPrepSkips(preps)
	skipped = append(skipped, plan.skipped...)

	if aborted {
		return result.FixReport{
			StartedAt:    startedAt,
			EndedAt:      time.Now(),
			Aborted:      true,
			DryRun:       opts.DryRun,
			SkipBackup:   opts.SkipBackup,
			Force:        opts.Force,
			FixSelectors: firstNonEmptyFixSelectors(preps),
			Plan:         plannedFixes,
			Skipped:      skipped,
			Diff:         diff,
			Projects:     projectSummaries,
		}, nil
	}

	approvedSet := approvedIDSet(plan.approvedIDs)

	var allItems []result.FixItem
	var allDiffs []lockdiff.FileDiff

	backupDirs := make(map[string]string)

	progress.Plan(filterApprovedCandidates(allCandidates, approvedSet))

	for _, prep := range preps {
		if prep.prepError != nil {
			continue
		}

		projectApprovedSpecs := filterSpecsByProjectApprovals(prep.specs, prep.candidates, approvedSet)

		if len(projectApprovedSpecs) == 0 {
			continue
		}

		backupDir, backupErr := tryBackupLockFiles(prep.rc, projectApprovedSpecs, opts)

		if backupErr != nil {
			allItems = append(allItems, backupFailureItem(prep.project.RelativePath, backupErr))
			continue
		}

		if backupDir != "" {
			backupDirs[prep.project.RelativePath] = backupDir
		}

		items := runApprovedFixers(ctx, projectApprovedSpecs, prep.candidates, prep.rc, opts, progress)

		for i := range items {
			items[i].Project = prep.project.RelativePath
		}

		allItems = append(allItems, items...)

		if diff && !opts.DryRun && backupDir != "" {
			projectDiffs := computeLockDiffs(prep.rc, backupDir)

			for i := range projectDiffs {
				projectDiffs[i].File = filepath.ToSlash(filepath.Join(prep.project.RelativePath, projectDiffs[i].File))
			}

			allDiffs = append(allDiffs, projectDiffs...)
		}
	}

	return result.FixReport{
		StartedAt:    startedAt,
		EndedAt:      time.Now(),
		Canceled:     ctx.Err() != nil,
		DryRun:       opts.DryRun,
		SkipBackup:   opts.SkipBackup,
		BackupDirs:   backupDirs,
		Force:        opts.Force,
		FixSelectors: firstNonEmptyFixSelectors(preps),
		Plan:         plannedFixes,
		Items:        allItems,
		Skipped:      skipped,
		Diff:         diff,
		LockDiffs:    allDiffs,
		Projects:     projectSummaries,
	}, nil
}

func (r Runner) prepareMonorepoProjects(projects []monorepo.Project, only []string) ([]projectFixPrep, error) {
	preps := make([]projectFixPrep, 0, len(projects))

	for _, project := range projects {
		prep := projectFixPrep{project: project}

		selection, err := Select(SelectInput{Only: only, Mode: ModeFix})
		if err != nil {
			return nil, err
		}

		prep.selection = selection

		rc := r.runContextForDir(project.AbsolutePath)
		prep.rc = rc

		if err := validateRequestedPackageManagers(only, rc); err != nil {
			prep.prepError = err
			prep.skippedOnPrepFail = result.SkippedFix{
				Project: project.RelativePath,
				Reason:  err.Error(),
			}
			preps = append(preps, prep)
			continue
		}

		specs := filterComposerUnlessExplicit(selection.Specs, rc, only)
		prep.specs = specs

		candidates := buildFixCandidates(specs, rc)

		for i := range candidates {
			candidates[i].Project = project.RelativePath
		}

		prep.candidates = candidates

		preps = append(preps, prep)
	}

	return preps, nil
}

func (r Runner) fixSingleProject(
	ctx context.Context,
	workDir string,
	projectPath string,
	only []string,
	opts ecosystem.FixOptions,
	diff bool,
	approver FixApprover,
	progress FixProgress,
) (result.FixReport, error) {
	selection, err := Select(SelectInput{Only: only, Mode: ModeFix})
	if err != nil {
		return result.FixReport{}, err
	}

	rc := r.runContextForDir(workDir)

	if err := validateRequestedPackageManagers(only, rc); err != nil {
		return result.FixReport{}, err
	}

	specs := filterComposerUnlessExplicit(selection.Specs, rc, only)
	startedAt := time.Now()

	candidates := buildFixCandidates(specs, rc)

	for i := range candidates {
		candidates[i].Project = projectPath
	}

	plannedFixes := plannedFixesFromCandidates(candidates)
	plan, aborted, err := resolveFixPlan(candidates, approver)
	if err != nil {
		return result.FixReport{}, fmt.Errorf("approval failed: %w", err)
	}

	if aborted {
		return result.FixReport{
			StartedAt:    startedAt,
			EndedAt:      time.Now(),
			Aborted:      true,
			DryRun:       opts.DryRun,
			SkipBackup:   opts.SkipBackup,
			Force:        opts.Force,
			FixSelectors: selection.FixSelectors,
			Plan:         plannedFixes,
			Skipped:      plan.skipped,
			Diff:         diff,
		}, nil
	}

	approvedSpecs := filterSpecsByIDs(specs, plan.approvedIDs)

	var backupDir string

	if !opts.DryRun && !opts.SkipBackup && len(approvedSpecs) > 0 {
		dir, err := backupSelectedLockFiles(rc, approvedSpecs)
		if err != nil {
			return result.FixReport{}, fmt.Errorf("failed to backup lock files: %w", err)
		}

		backupDir = dir
	}

	progress.Plan(filterApprovedCandidates(candidates, approvedIDSet(plan.approvedIDs)))

	items := runApprovedFixers(ctx, approvedSpecs, candidates, rc, opts, progress)

	var diffs []lockdiff.FileDiff

	if diff && !opts.DryRun && backupDir != "" {
		diffs = computeLockDiffs(rc, backupDir)
	}

	return result.FixReport{
		StartedAt:    startedAt,
		EndedAt:      time.Now(),
		Canceled:     ctx.Err() != nil,
		DryRun:       opts.DryRun,
		SkipBackup:   opts.SkipBackup,
		BackupDir:    backupDir,
		Force:        opts.Force,
		FixSelectors: selection.FixSelectors,
		Plan:         plannedFixes,
		Items:        items,
		Skipped:      plan.skipped,
		Diff:         diff,
		LockDiffs:    diffs,
	}, nil
}

func collectPrepSkips(preps []projectFixPrep) []result.SkippedFix {
	var skipped []result.SkippedFix

	for _, prep := range preps {
		if prep.prepError != nil {
			skipped = append(skipped, prep.skippedOnPrepFail)
		}
	}

	return skipped
}

func firstNonEmptyFixSelectors(preps []projectFixPrep) []string {
	for _, prep := range preps {
		if len(prep.selection.FixSelectors) > 0 {
			return prep.selection.FixSelectors
		}
	}

	return nil
}

func tryBackupLockFiles(rc ecosystem.RunContext, specs []*ecosystem.Spec, opts ecosystem.FixOptions) (string, error) {
	if opts.DryRun || opts.SkipBackup || len(specs) == 0 {
		return "", nil
	}

	return backupSelectedLockFiles(rc, specs)
}

func backupFailureItem(projectPath string, err error) result.FixItem {
	now := time.Now()

	return result.FixItem{
		Project:     projectPath,
		ScopeID:     "",
		ManagerName: "Backup",
		Success:     false,
		Error:       fmt.Sprintf("lock file backup failed: %v", err),
		StartedAt:   now,
		EndedAt:     now,
	}
}

func approvedIDSet(approvedIDs []string) map[string]struct{} {
	set := make(map[string]struct{}, len(approvedIDs))

	for _, id := range approvedIDs {
		set[id] = struct{}{}
	}

	return set
}

func filterApprovedCandidates(candidates []FixCandidate, approved map[string]struct{}) []FixCandidate {
	filtered := make([]FixCandidate, 0, len(candidates))

	for _, candidate := range candidates {
		if _, ok := approved[candidateApprovalKey(candidate)]; ok {
			filtered = append(filtered, candidate)
		}
	}

	return filtered
}

func filterSpecsByProjectApprovals(specs []*ecosystem.Spec, candidates []FixCandidate, approved map[string]struct{}) []*ecosystem.Spec {
	projectScopeApproved := make(map[string]struct{}, len(candidates))

	for _, candidate := range candidates {
		if _, ok := approved[candidateApprovalKey(candidate)]; ok {
			projectScopeApproved[candidate.ScopeID] = struct{}{}
		}
	}

	filtered := make([]*ecosystem.Spec, 0, len(projectScopeApproved))

	for _, spec := range specs {
		if _, ok := projectScopeApproved[spec.Name]; ok {
			filtered = append(filtered, spec)
		}
	}

	return filtered
}

func candidateApprovalKey(candidate FixCandidate) string {
	return candidate.Project + "\x00" + candidate.ScopeID
}

func plannedFixesFromCandidates(candidates []FixCandidate) []result.PlannedFix {
	if len(candidates) == 0 {
		return nil
	}

	planned := make([]result.PlannedFix, 0, len(candidates))

	for _, candidate := range candidates {
		planned = append(planned, result.PlannedFix{
			Project:     candidate.Project,
			ScopeID:     candidate.ScopeID,
			DisplayName: candidate.DisplayName,
			Command:     candidate.Command,
			Summary:     candidate.Summary,
		})
	}

	return planned
}

func runApprovedFixers(
	ctx context.Context,
	specs []*ecosystem.Spec,
	candidates []FixCandidate,
	rc ecosystem.RunContext,
	opts ecosystem.FixOptions,
	progress FixProgress,
) []result.FixItem {
	items := make([]result.FixItem, 0, len(specs))
	candidateByID := candidatesByScopeID(candidates)

	for _, spec := range specs {
		candidate := candidateByID[spec.Name]
		progress.Start(candidate)

		startedAt := time.Now()
		fixItem, fixErr := fixSpec(ctx, spec, rc, opts)
		endedAt := time.Now()

		var item result.FixItem

		if fixErr != nil {
			item = result.FixItem{
				ScopeID:     spec.Name,
				ManagerName: spec.Title(),
				Success:     false,
				Error:       fixErr.Error(),
				StartedAt:   startedAt,
				EndedAt:     endedAt,
			}
		} else {
			item = result.FromFixItem(fixItem, startedAt, endedAt)
		}

		item.Project = candidate.Project

		progress.Finish(item)
		items = append(items, item)
	}

	return items
}

func fixSpec(ctx context.Context, spec *ecosystem.Spec, rc ecosystem.RunContext, opts ecosystem.FixOptions) (ecosystem.FixItem, error) {
	detection, ok := spec.Resolve(rc)

	if !ok {
		return ecosystem.FixItem{ScopeID: spec.Name, Success: true}, nil
	}

	return spec.RunFix(ctx, rc, detection, opts)
}

func candidatesByScopeID(candidates []FixCandidate) map[string]FixCandidate {
	indexed := make(map[string]FixCandidate, len(candidates))

	for _, candidate := range candidates {
		indexed[candidate.ScopeID] = candidate
	}

	return indexed
}

type fixPlan struct {
	approvedIDs []string
	skipped     []result.SkippedFix
}

func resolveFixPlan(candidates []FixCandidate, approver FixApprover) (fixPlan, bool, error) {
	plan := fixPlan{approvedIDs: make([]string, 0, len(candidates))}

	for _, candidate := range candidates {
		decision, err := approver.Approve(candidate)
		if err != nil {
			return fixPlan{}, false, err
		}

		switch decision {
		case FixApply:
			plan.approvedIDs = append(plan.approvedIDs, candidateApprovalKey(candidate))
		case FixSkip:
			plan.skipped = append(plan.skipped, skippedFrom(candidate, "declined by user"))
		case FixAbort:
			plan.skipped = append(plan.skipped, skippedFrom(candidate, "aborted by user"))
			return plan, true, nil
		}
	}

	return plan, false, nil
}

func skippedFrom(candidate FixCandidate, reason string) result.SkippedFix {
	return result.SkippedFix{
		Project:     candidate.Project,
		ScopeID:     candidate.ScopeID,
		DisplayName: candidate.DisplayName,
		Command:     candidate.Command,
		Reason:      reason,
	}
}

func filterSpecsByIDs(specs []*ecosystem.Spec, approvedIDs []string) []*ecosystem.Spec {
	if len(approvedIDs) == 0 {
		return nil
	}

	approvedScopes := make(map[string]struct{}, len(approvedIDs))

	for _, id := range approvedIDs {
		if _, after, ok := strings.Cut(id, "\x00"); ok {
			approvedScopes[after] = struct{}{}
		} else {
			approvedScopes[id] = struct{}{}
		}
	}

	filtered := make([]*ecosystem.Spec, 0, len(approvedScopes))

	for _, spec := range specs {
		if _, ok := approvedScopes[spec.Name]; ok {
			filtered = append(filtered, spec)
		}
	}

	return filtered
}

func buildFixCandidates(specs []*ecosystem.Spec, rc ecosystem.RunContext) []FixCandidate {
	candidates := make([]FixCandidate, 0, len(specs))

	for _, spec := range specs {
		if !spec.CanFix() {
			continue
		}

		detection, ok := spec.Resolve(rc)

		if !ok {
			continue
		}

		manager := detection.Active
		command := strings.TrimSpace(manager.Command + " " + strings.Join(manager.InstallArgs, " "))

		candidates = append(candidates, FixCandidate{
			ScopeID:     spec.Name,
			DisplayName: spec.Title(),
			Command:     command,
			Summary:     candidateSummary(manager, rc),
		})
	}

	return candidates
}

func candidateSummary(manager ecosystem.Manager, rc ecosystem.RunContext) string {
	if manager.LockFile != "" && rc.FileExists(manager.LockFile) {
		return "sync " + manager.ConfigFile + " + " + manager.LockFile
	}

	return "install from " + manager.ConfigFile
}

func computeLockDiffs(rc ecosystem.RunContext, backupDir string) []lockdiff.FileDiff {
	var diffs []lockdiff.FileDiff

	for _, filename := range lockdiff.RegisteredFilenames() {
		backupBytes, err := rc.FS.ReadFile(filepath.Join(backupDir, filename))
		if err != nil {
			continue
		}

		parser, ok := lockdiff.ParserFor(filename)

		if !ok {
			continue
		}

		before, err := parser.Parse(backupBytes)
		if err != nil {
			continue
		}

		after := map[string]string{}

		if currentBytes, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, filename)); err == nil {
			if parsed, parseErr := parser.Parse(currentBytes); parseErr == nil {
				after = parsed
			}
		}

		diff := lockdiff.Diff(parser.Ecosystem(), filename, before, after)

		if !diff.Empty() {
			diffs = append(diffs, diff)
		}
	}

	return diffs
}

func backupSelectedLockFiles(rc ecosystem.RunContext, specs []*ecosystem.Spec) (string, error) {
	if len(specs) == 0 {
		return "", nil
	}

	backupDir := filepath.Join(rc.WorkDir, ".preflight", "backups", time.Now().Format("20060102-150405"))

	if err := rc.FS.MkdirAll(backupDir, 0o750); err != nil {
		return "", err
	}

	for _, lock := range collectLockFilesForBackup(rc, specs) {
		src, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, lock))
		if err != nil {
			return "", err
		}

		dst := filepath.Join(backupDir, lock)

		if err := rc.FS.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return "", err
		}

		if err := rc.FS.WriteFile(dst, src, 0o600); err != nil {
			return "", err
		}
	}

	return backupDir, nil
}

func collectLockFilesForBackup(rc ecosystem.RunContext, specs []*ecosystem.Spec) []string {
	var lockFiles []string

	for _, spec := range specs {
		detection, ok := spec.Resolve(rc)

		if !ok {
			continue
		}

		lock := detection.Active.LockFile

		if lock == "" || !rc.FileExists(lock) {
			continue
		}

		lockFiles = append(lockFiles, lock)
	}

	return lockFiles
}
