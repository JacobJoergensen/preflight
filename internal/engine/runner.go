package engine

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/manifest"
)

type Runner struct {
	WorkDir       string
	FS            fs.FS
	Command       exec.Runner
	CommandStream exec.StreamRunner
}

func NewRunner(workDir string) Runner {
	return Runner{
		WorkDir:       workDir,
		FS:            fs.OSFS{},
		Command:       exec.DefaultRunner{},
		CommandStream: exec.DefaultStreamRunner{},
	}
}

func (r Runner) deps() adapter.Dependencies {
	loader := manifest.NewLoader(r.WorkDir)
	loader.FS = r.FS

	return adapter.Dependencies{
		Loader: loader,
		FS:     r.FS,
		Runner: r.Command,
		Stream: r.CommandStream,
	}
}

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

func appendEnvIfRequested(adapters []adapter.Adapter, withEnv bool, scopes, selectors []string) []adapter.Adapter {
	if !withEnv || selectionIncludesEnv(scopes, selectors) {
		return adapters
	}

	envAdapters, err := adapter.Select("env")

	if err != nil || len(envAdapters) == 0 {
		return adapters
	}

	return append(adapters, envAdapters[0])
}

func selectionIncludesEnv(scopes, selectors []string) bool {
	for _, s := range scopes {
		if strings.EqualFold(strings.TrimSpace(s), "env") {
			return true
		}
	}

	for _, s := range selectors {
		if strings.EqualFold(strings.TrimSpace(s), "env") {
			return true
		}
	}

	return false
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

// Skips Composer on implicit selection when composer.json is absent to avoid stray lockfile warnings.
func filterComposerUnlessExplicit(adapters []adapter.Adapter, deps adapter.Dependencies, scopes, selectors []string) []adapter.Adapter {
	if !isImplicitFullSelection(scopes, selectors) {
		return adapters
	}

	if deps.Loader.HasComposerJSON() {
		return adapters
	}

	return withoutAdapter(adapters, "composer")
}

func isImplicitFullSelection(scopes, selectors []string) bool {
	return len(nonEmptyStrings(scopes)) == 0 && len(nonEmptyStrings(selectors)) == 0
}

func nonEmptyStrings(ss []string) []string {
	output := make([]string, 0, len(ss))

	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			output = append(output, s)
		}
	}

	return output
}

func withoutAdapter(adapters []adapter.Adapter, name string) []adapter.Adapter {
	return slices.DeleteFunc(slices.Clone(adapters), func(a adapter.Adapter) bool {
		return a.Name() == name
	})
}

func validateRequestedPackageManagers(selectors []string, deps adapter.Dependencies) error {
	for _, selector := range selectors {
		selector = strings.ToLower(strings.TrimSpace(selector))

		if selector == "" {
			continue
		}

		// Skip if it's a scope (js, python, etc.) rather than a specific PM
		if manifest.IsPackageType(selector) {
			continue
		}

		packageType, isPackageManager := manifest.GetPackageType(selector)

		if !isPackageManager {
			continue
		}

		// Only validate for package types with multiple managers (js, python)
		if packageType != manifest.PackageTypeJS && packageType != manifest.PackageTypePython {
			continue
		}

		detected, ok := deps.Loader.DetectPackageManager(packageType)

		if !ok {
			return fmt.Errorf("requested %s but no %s project detected", selector, packageType)
		}

		if !strings.EqualFold(detected.Command(), selector) {
			return fmt.Errorf("requested %s but project uses %s", selector, detected.Command())
		}
	}

	return nil
}

func runParallel[Job, Result any](ctx context.Context, jobs []Job, work func(context.Context, Job) (Result, bool)) []Result {
	if len(jobs) == 0 {
		return nil
	}

	results := make(chan Result, len(jobs))
	jobsCh := make(chan Job)
	workerCount := parallelWorkerCount(len(jobs))

	var wg sync.WaitGroup

	wg.Add(workerCount)

	for range workerCount {
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobsCh:
					if !ok {
						return
					}

					item, include := work(ctx, job)

					if !include {
						continue
					}

					select {
					case results <- item:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobsCh)

		for _, job := range jobs {
			select {
			case jobsCh <- job:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	out := make([]Result, 0, len(jobs))

	for item := range results {
		out = append(out, item)
	}

	return out
}

func parallelWorkerCount(jobCount int) int {
	if jobCount <= 0 {
		return 1
	}

	const maxWorkers = 8

	workers := runtime.GOMAXPROCS(0)
	workers = max(workers, 1)
	workers = min(workers, maxWorkers)
	workers = min(workers, jobCount)

	return workers
}

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
		Dependencies: depsByAdapter,
		Outdated:     outdatedByAdapter,
		Elapsed:      elapsedByAdapter,
	}, nil
}

func (r Runner) Fix(ctx context.Context, scopes []string, selectors []string, opts adapter.FixOptions) (result.FixReport, error) {
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

	var backupDir string

	if !opts.DryRun && !opts.SkipBackup {
		dir, err := backupSelectedLockFiles(deps, adapter.Names(adapters), selection.FixSelectors)

		if err != nil {
			return result.FixReport{
				StartedAt:     startedAt,
				EndedAt:       time.Now(),
				DryRun:        opts.DryRun,
				SkipBackup:    opts.SkipBackup,
				Force:         opts.Force,
				FixSelectors:  selection.FixSelectors,
				InternalError: fmt.Sprintf("failed to backup lock files: %v", err),
			}, fmt.Errorf("failed to backup lock files: %w", err)
		}

		backupDir = dir
	}

	items := make([]result.FixItem, 0, len(adapters))

	for _, a := range adapters {
		fixer, ok := a.(adapter.Fixer)

		if !ok {
			continue
		}

		item, itemErr := fixer.Fix(ctx, deps, selection.FixSelectors, opts)
		now := time.Now()

		if itemErr != nil {
			items = append(items, result.FixItem{
				ScopeID:     a.Name(),
				ManagerName: adapter.DisplayName(a),
				Success:     false,
				Error:       itemErr.Error(),
				StartedAt:   now,
				EndedAt:     now,
			})
			continue
		}

		items = append(items, result.FromAdapterFix(item, now, now))
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
		Items:        items,
	}, nil
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

	var lockFiles []string

	if _, ok := want["js"]; ok {
		packageManager, ok := deps.Loader.DetectPackageManager(manifest.PackageTypeJS)

		if ok && packageManager.LockFileExists && packageManager.LockFile() != "" {
			if manifest.AnyMatchesPackageType(selectors, manifest.PackageTypeJS) && !slices.Contains(selectors, packageManager.Command()) {
				// user asked for npm/yarn/etc but detected another, skip backup
			} else {
				lockFiles = append(lockFiles, packageManager.LockFile())
			}
		}
	}

	if _, ok := want["composer"]; ok {
		packageManager, ok := deps.Loader.DetectPackageManager(manifest.PackageTypeComposer)

		if ok && packageManager.LockFileExists && packageManager.LockFile() != "" {
			lockFiles = append(lockFiles, packageManager.LockFile())
		}
	}

	if _, ok := want["go"]; ok {
		packageManager, ok := deps.Loader.DetectPackageManager(manifest.PackageTypeGo)

		if ok && packageManager.LockFileExists && packageManager.LockFile() != "" {
			lockFiles = append(lockFiles, packageManager.LockFile())
		}
	}

	if _, ok := want["python"]; ok {
		for _, name := range []string{"poetry.lock", "uv.lock", "Pipfile.lock", "pdm.lock"} {
			srcPath := filepath.Join(deps.Loader.WorkDir, name)

			if _, err := deps.FS.Stat(srcPath); err != nil {
				continue
			}

			lockFiles = append(lockFiles, name)
		}
	}

	if _, ok := want["ruby"]; ok {
		packageManager, ok := deps.Loader.DetectPackageManager(manifest.PackageTypeRuby)

		if ok && packageManager.LockFileExists && packageManager.LockFile() != "" {
			lockFiles = append(lockFiles, packageManager.LockFile())
		}
	}

	for _, lock := range lockFiles {
		srcPath := filepath.Join(deps.Loader.WorkDir, lock)
		src, err := deps.FS.ReadFile(srcPath)

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
