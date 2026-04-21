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
)

type FixCandidate struct {
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
) (result.FixReport, error) {
	if approver == nil {
		approver = AutoFixApprover{}
	}

	if progress == nil {
		progress = NoopFixProgress{}
	}

	selection, err := Select(SelectInput{Scopes: scopes, Selectors: selectors, Mode: ModeFix})

	if err != nil {
		return result.FixReport{}, err
	}

	deps := r.deps()

	if err := validateRequestedPackageManagers(selectors, deps); err != nil {
		return result.FixReport{}, err
	}

	adapters := filterComposerUnlessExplicit(selection.Adapters, deps, scopes, selectors)
	startedAt := time.Now()

	candidates := buildFixCandidates(adapters, deps)
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

func plannedFixesFromCandidates(candidates []FixCandidate) []result.PlannedFix {
	if len(candidates) == 0 {
		return nil
	}

	planned := make([]result.PlannedFix, 0, len(candidates))

	for _, candidate := range candidates {
		planned = append(planned, result.PlannedFix{
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
	approvedCandidates := approvedCandidatesForAdapters(adapters, candidateByID)

	progress.Plan(approvedCandidates)

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

func approvedCandidatesForAdapters(adapters []adapter.Adapter, candidateByID map[string]FixCandidate) []FixCandidate {
	approved := make([]FixCandidate, 0, len(adapters))

	for _, a := range adapters {
		if candidate, ok := candidateByID[a.Name()]; ok {
			approved = append(approved, candidate)
		}
	}

	return approved
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
			plan.approvedIDs = append(plan.approvedIDs, candidate.ScopeID)
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

	approved := make(map[string]struct{}, len(approvedIDs))

	for _, id := range approvedIDs {
		approved[id] = struct{}{}
	}

	filtered := make([]adapter.Adapter, 0, len(approvedIDs))

	for _, a := range adapters {
		if _, ok := approved[a.Name()]; ok {
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
