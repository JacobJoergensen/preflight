package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
	"github.com/JacobJoergensen/preflight/internal/manifest"
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
	scopes []string,
	selectors []string,
	opts adapter.FixOptions,
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

	if !disableMonorepo {
		projects, err := monorepo.DiscoverProjects(r.WorkDir)

		if err != nil {
			return result.FixReport{}, fmt.Errorf("monorepo discovery failed: %w", err)
		}

		projects, err = monorepo.FilterByGlobs(projects, projectGlobs)

		if err != nil {
			return result.FixReport{}, fmt.Errorf("project filter failed: %w", err)
		}

		if len(projects) > 0 {
			return r.fixMonorepo(ctx, projects, scopes, selectors, opts, diff, approver, progress)
		}
	}

	return r.fixSingleProject(ctx, r.WorkDir, "", scopes, selectors, opts, diff, approver, progress)
}

type projectFixPrep struct {
	project           monorepo.Project
	deps              adapter.Dependencies
	adapters          []adapter.Adapter
	selection         Selection
	candidates        []FixCandidate
	prepError         error
	skippedOnPrepFail result.SkippedFix
}

func (r Runner) fixMonorepo(
	ctx context.Context,
	projects []monorepo.Project,
	scopes []string,
	selectors []string,
	opts adapter.FixOptions,
	diff bool,
	approver FixApprover,
	progress FixProgress,
) (result.FixReport, error) {
	startedAt := time.Now()

	projectSummaries := make([]result.FixProject, 0, len(projects))

	for _, project := range projects {
		projectSummaries = append(projectSummaries, result.FixProject{
			RelativePath: project.RelativePath,
			Name:         project.Name,
		})
	}

	preps, fatalErr := r.prepareMonorepoProjects(projects, scopes, selectors)

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

		projectApprovedAdapters := filterAdaptersByProjectApprovals(prep.adapters, prep.candidates, approvedSet)

		if len(projectApprovedAdapters) == 0 {
			continue
		}

		backupDir, backupErr := tryBackupLockFiles(prep.deps, projectApprovedAdapters, prep.selection.FixSelectors, opts)

		if backupErr != nil {
			allItems = append(allItems, backupFailureItem(prep.project.RelativePath, backupErr))
			continue
		}

		if backupDir != "" {
			backupDirs[prep.project.RelativePath] = backupDir
		}

		items := runApprovedFixers(ctx, projectApprovedAdapters, prep.candidates, prep.deps, prep.selection.FixSelectors, opts, progress)

		for i := range items {
			items[i].Project = prep.project.RelativePath
		}

		allItems = append(allItems, items...)

		if diff && !opts.DryRun && backupDir != "" {
			projectDiffs := computeLockDiffs(prep.deps, backupDir)

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

func (r Runner) prepareMonorepoProjects(projects []monorepo.Project, scopes, selectors []string) ([]projectFixPrep, error) {
	preps := make([]projectFixPrep, 0, len(projects))

	for _, project := range projects {
		prep := projectFixPrep{project: project}

		selection, err := Select(SelectInput{Scopes: scopes, Selectors: selectors, Mode: ModeFix})

		if err != nil {
			return nil, err
		}

		prep.selection = selection

		deps := r.depsForDir(project.AbsolutePath)
		prep.deps = deps

		if err := validateRequestedPackageManagers(selectors, deps); err != nil {
			prep.prepError = err
			prep.skippedOnPrepFail = result.SkippedFix{
				Project: project.RelativePath,
				Reason:  err.Error(),
			}
			preps = append(preps, prep)
			continue
		}

		adapters := filterComposerUnlessExplicit(selection.Adapters, deps, scopes, selectors)
		prep.adapters = adapters

		candidates := buildFixCandidates(adapters, deps)

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
	scopes []string,
	selectors []string,
	opts adapter.FixOptions,
	diff bool,
	approver FixApprover,
	progress FixProgress,
) (result.FixReport, error) {
	selection, err := Select(SelectInput{Scopes: scopes, Selectors: selectors, Mode: ModeFix})

	if err != nil {
		return result.FixReport{}, err
	}

	deps := r.depsForDir(workDir)

	if err := validateRequestedPackageManagers(selectors, deps); err != nil {
		return result.FixReport{}, err
	}

	adapters := filterComposerUnlessExplicit(selection.Adapters, deps, scopes, selectors)
	startedAt := time.Now()

	candidates := buildFixCandidates(adapters, deps)

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

	approvedAdapters := filterAdaptersByIDs(adapters, plan.approvedIDs)

	var backupDir string

	if !opts.DryRun && !opts.SkipBackup && len(approvedAdapters) > 0 {
		dir, err := backupSelectedLockFiles(deps, adapter.Names(approvedAdapters), selection.FixSelectors)

		if err != nil {
			return result.FixReport{}, fmt.Errorf("failed to backup lock files: %w", err)
		}

		backupDir = dir
	}

	progress.Plan(filterApprovedCandidates(candidates, approvedIDSet(plan.approvedIDs)))

	items := runApprovedFixers(ctx, approvedAdapters, candidates, deps, selection.FixSelectors, opts, progress)

	var diffs []lockdiff.FileDiff

	if diff && !opts.DryRun && backupDir != "" {
		diffs = computeLockDiffs(deps, backupDir)
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

func tryBackupLockFiles(deps adapter.Dependencies, adapters []adapter.Adapter, selectors []string, opts adapter.FixOptions) (string, error) {
	if opts.DryRun || opts.SkipBackup || len(adapters) == 0 {
		return "", nil
	}

	return backupSelectedLockFiles(deps, adapter.Names(adapters), selectors)
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

func filterAdaptersByProjectApprovals(adapters []adapter.Adapter, candidates []FixCandidate, approved map[string]struct{}) []adapter.Adapter {
	projectScopeApproved := make(map[string]struct{}, len(candidates))

	for _, candidate := range candidates {
		if _, ok := approved[candidateApprovalKey(candidate)]; ok {
			projectScopeApproved[candidate.ScopeID] = struct{}{}
		}
	}

	filtered := make([]adapter.Adapter, 0, len(projectScopeApproved))

	for _, a := range adapters {
		if _, ok := projectScopeApproved[a.Name()]; ok {
			filtered = append(filtered, a)
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
	adapters []adapter.Adapter,
	candidates []FixCandidate,
	deps adapter.Dependencies,
	fixSelectors []string,
	opts adapter.FixOptions,
	progress FixProgress,
) []result.FixItem {
	items := make([]result.FixItem, 0, len(adapters))
	candidateByID := candidatesByScopeID(candidates)

	for _, a := range adapters {
		candidate := candidateByID[a.Name()]
		progress.Start(candidate)

		startedAt := time.Now()
		adapterItem, fixErr := a.(adapter.Fixer).Fix(ctx, deps, fixSelectors, opts)
		endedAt := time.Now()

		var item result.FixItem

		if fixErr != nil {
			item = result.FixItem{
				ScopeID:     a.Name(),
				ManagerName: adapter.DisplayName(a),
				Success:     false,
				Error:       fixErr.Error(),
				StartedAt:   startedAt,
				EndedAt:     endedAt,
			}
		} else {
			item = result.FromAdapterFix(adapterItem, startedAt, endedAt)
		}

		item.Project = candidate.Project

		progress.Finish(item)
		items = append(items, item)
	}

	return items
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

func filterAdaptersByIDs(adapters []adapter.Adapter, approvedIDs []string) []adapter.Adapter {
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

	filtered := make([]adapter.Adapter, 0, len(approvedScopes))

	for _, a := range adapters {
		if _, ok := approvedScopes[a.Name()]; ok {
			filtered = append(filtered, a)
		}
	}

	return filtered
}

func buildFixCandidates(adapters []adapter.Adapter, deps adapter.Dependencies) []FixCandidate {
	candidates := make([]FixCandidate, 0, len(adapters))

	for _, a := range adapters {
		if _, ok := a.(adapter.Fixer); !ok {
			continue
		}

		packageManager, ok := deps.Loader.DetectPackageManager(a.Name())

		if !ok || (!packageManager.ConfigFileExists && !packageManager.LockFileExists) {
			continue
		}

		command := strings.TrimSpace(packageManager.Command() + " " + strings.Join(packageManager.Tool.InstallArgs, " "))

		candidates = append(candidates, FixCandidate{
			ScopeID:     a.Name(),
			DisplayName: adapter.DisplayName(a),
			Command:     command,
			Summary:     candidateSummary(packageManager),
		})
	}

	return candidates
}

func candidateSummary(packageManager manifest.PackageManager) string {
	if packageManager.LockFileExists && packageManager.Tool.LockFile != "" {
		return "sync " + packageManager.Tool.ConfigFile + " + " + packageManager.Tool.LockFile
	}

	return "install from " + packageManager.Tool.ConfigFile
}

func computeLockDiffs(deps adapter.Dependencies, backupDir string) []lockdiff.FileDiff {
	var diffs []lockdiff.FileDiff

	for _, filename := range lockdiff.RegisteredFilenames() {
		backupBytes, err := deps.FS.ReadFile(filepath.Join(backupDir, filename))

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

		if currentBytes, err := deps.FS.ReadFile(filepath.Join(deps.Loader.WorkDir, filename)); err == nil {
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

func backupSelectedLockFiles(deps adapter.Dependencies, adapterIDs []string, selectors []string) (string, error) {
	want := make(map[string]struct{}, len(adapterIDs))

	for _, id := range adapterIDs {
		want[id] = struct{}{}
	}

	if len(want) == 0 {
		return "", nil
	}

	backupDir := filepath.Join(deps.Loader.WorkDir, ".preflight", "backups", time.Now().Format("20060102-150405"))

	if err := deps.FS.MkdirAll(backupDir, 0750); err != nil {
		return "", err
	}

	for _, lock := range collectLockFilesForBackup(deps, want, selectors) {
		src, err := deps.FS.ReadFile(filepath.Join(deps.Loader.WorkDir, lock))

		if err != nil {
			return "", err
		}

		dst := filepath.Join(backupDir, lock)

		if err := deps.FS.MkdirAll(filepath.Dir(dst), 0750); err != nil {
			return "", err
		}

		if err := deps.FS.WriteFile(dst, src, 0600); err != nil {
			return "", err
		}
	}

	return backupDir, nil
}

func collectLockFilesForBackup(deps adapter.Dependencies, want map[string]struct{}, selectors []string) []string {
	var lockFiles []string

	if _, ok := want["js"]; ok {
		packageManager, ok := deps.Loader.DetectPackageManager(manifest.PackageTypeJS)

		if ok && packageManager.LockFileExists && packageManager.LockFile() != "" {
			jsMismatch := manifest.AnyMatchesPackageType(selectors, manifest.PackageTypeJS) && !slices.Contains(selectors, packageManager.Command())

			if !jsMismatch {
				lockFiles = append(lockFiles, packageManager.LockFile())
			}
		}
	}

	for _, singleTool := range []string{"composer", "go", "ruby"} {
		if _, ok := want[singleTool]; !ok {
			continue
		}

		packageManager, ok := deps.Loader.DetectPackageManager(singleTool)

		if ok && packageManager.LockFileExists && packageManager.LockFile() != "" {
			lockFiles = append(lockFiles, packageManager.LockFile())
		}
	}

	if _, ok := want["python"]; ok {
		for _, name := range []string{"poetry.lock", "uv.lock", "Pipfile.lock", "pdm.lock"} {
			if _, err := deps.FS.Stat(filepath.Join(deps.Loader.WorkDir, name)); err == nil {
				lockFiles = append(lockFiles, name)
			}
		}
	}

	return lockFiles
}
