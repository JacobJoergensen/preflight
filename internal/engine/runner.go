package engine

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/JacobJoergensen/preflight/internal/adapter"
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

func (r Runner) depsForDir(workDir string) adapter.Dependencies {
	loader := manifest.NewLoader(workDir)
	loader.FS = r.FS

	return adapter.Dependencies{
		Loader: loader,
		FS:     r.FS,
		Runner: r.Command,
		Stream: r.CommandStream,
	}
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

	results := make([]Result, len(jobs))
	includes := make([]bool, len(jobs))
	semaphore := make(chan struct{}, parallelWorkerCount(len(jobs)))

	var wg sync.WaitGroup

	wg.Add(len(jobs))

	for index, job := range jobs {
		go func(index int, job Job) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}

			defer func() { <-semaphore }()

			if ctx.Err() != nil {
				return
			}

			item, include := work(ctx, job)

			if !include {
				return
			}

			results[index] = item
			includes[index] = true
		}(index, job)
	}

	wg.Wait()

	output := make([]Result, 0, len(jobs))

	for i, include := range includes {
		if include {
			output = append(output, results[i])
		}
	}

	return output
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
