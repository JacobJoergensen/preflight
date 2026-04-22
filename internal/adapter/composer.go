package adapter

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/manifest"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func init() {
	Register(ComposerModule{})
}

type ComposerModule struct{}

func (c ComposerModule) Name() string {
	return "composer"
}

func (c ComposerModule) DisplayName() string {
	return "Composer"
}

func (c ComposerModule) Check(ctx context.Context, deps Dependencies) ([]Message, []Message, []Message) {
	errs := []Message{}
	warns := []Message{}
	succs := []Message{}

	if ctx.Err() != nil {
		return errs, warns, succs
	}

	composerConfig := deps.Loader.LoadComposerConfig()
	packageManager := composerConfig.PackageManager

	if packageManager.LockFile() == "" && !composerConfig.HasConfig {
		return errs, warns, succs
	}

	if !composerConfig.HasConfig {
		warns = append(warns, warnsMissingComposerJSON(packageManager.LockFile())...)
		return errs, warns, succs
	}

	if composerConfig.Error != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Failed to read composer.json: %v", composerConfig.Error)})
		return errs, warns, succs
	}

	composerVersion, err := getComposerVersion(ctx, deps.Runner)

	if err != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Composer is not installed or not on PATH: %v", err)})
		return errs, warns, succs
	}

	succs = append(succs, Message{Text: fmt.Sprintf("Installed %sComposer (%s)", terminal.Reset, composerVersion)})

	succs = append(succs, Message{Text: "composer.json found:"})
	installedDependencies := getInstalledDependencies(ctx, deps.Runner, composerConfig.Dependencies, composerConfig.DevDependencies)

	for _, dep := range append(composerConfig.Dependencies, composerConfig.DevDependencies...) {
		isDev := slices.Contains(composerConfig.DevDependencies, dep)

		if version, exists := installedDependencies[dep]; exists {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed dependency %s%s (%s)", terminal.Reset, dep, version), Nested: true, Dev: isDev})
		} else {
			errs = append(errs, Message{Text: fmt.Sprintf("Missing dependency %s%s, Run `composer require %s`", terminal.Reset, dep, dep), Nested: true, Dev: isDev})
		}
	}

	return errs, warns, succs
}

func (c ComposerModule) ListDependencies(ctx context.Context, deps Dependencies) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadComposerConfig()

	if !config.HasConfig || config.Error != nil {
		return nil, config.Error
	}

	return append(slices.Clone(config.Dependencies), config.DevDependencies...), nil
}

func (c ComposerModule) ListOutdated(ctx context.Context, deps Dependencies) ([]OutdatedPackage, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadComposerConfig()

	if !config.HasConfig {
		return nil, nil
	}

	output, err := deps.Runner.Run(ctx, "composer", "outdated", "--direct", "--format=json")

	if err != nil && output == "" {
		return nil, err
	}

	return parseComposerOutdated(output)
}

func parseComposerOutdated(output string) ([]OutdatedPackage, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	var data struct {
		Installed []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Latest  string `json:"latest"`
		} `json:"installed"`
	}

	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, err
	}

	packages := make([]OutdatedPackage, 0, len(data.Installed))

	for _, pkg := range data.Installed {
		if pkg.Version == pkg.Latest {
			continue
		}

		packages = append(packages, OutdatedPackage{
			Name:    pkg.Name,
			Current: pkg.Version,
			Latest:  pkg.Latest,
		})
	}

	slices.SortFunc(packages, func(a, b OutdatedPackage) int {
		return strings.Compare(a.Name, b.Name)
	})

	return packages, nil
}

func (c ComposerModule) Fix(ctx context.Context, deps Dependencies, _ []string, options FixOptions) (FixItem, error) {
	return fixByPackageType(ctx, deps, c.Name(), manifest.PackageTypeComposer, options)
}

func warnsMissingComposerJSON(lockFile string) []Message {
	warns := []Message{{Text: "composer.json not found."}}

	if lockFile != "" {
		warns = append(warns, Message{Text: fmt.Sprintf(
			"composer.json not found, but %s exists. Ensure composer.json is included in your project.",
			lockFile,
		)})
	}

	return warns
}

func getComposerVersion(ctx context.Context, runner exec.Runner) (string, error) {
	output, err := runner.Run(ctx, "composer", "--version")

	if err != nil {
		return "", fmt.Errorf("failed to run composer --version: %w", err)
	}

	parts := strings.Fields(strings.TrimSpace(output))

	if len(parts) >= 3 {
		return parts[2], nil
	}

	return "", fmt.Errorf("unexpected composer version format: %s", output)
}

func getInstalledDependencies(ctx context.Context, runner exec.Runner, dependencies, devDependencies []string) map[string]string {
	allDeps := append(dependencies, devDependencies...)

	installed := composerInstalledFromJSON(ctx, runner)

	if len(allDeps) == 0 {
		return installed
	}

	fillMissingComposerDeps(ctx, runner, installed, allDeps)
	return installed
}

func composerInstalledFromJSON(ctx context.Context, runner exec.Runner) map[string]string {
	installed := make(map[string]string)
	output, err := runner.Run(ctx, "composer", "show", "--format=json")

	if err != nil {
		return installed
	}

	var data struct {
		Dependencies []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"installed"`
	}

	if json.Unmarshal([]byte(output), &data) != nil {
		return installed
	}

	for _, dependency := range data.Dependencies {
		if dependency.Name == "" {
			continue
		}

		installed[dependency.Name] = dependency.Version
	}

	return installed
}

func fillMissingComposerDeps(ctx context.Context, runner exec.Runner, installed map[string]string, required []string) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, dep := range required {
		if dep == "" {
			continue
		}

		if _, exists := installed[dep]; exists {
			continue
		}

		wg.Add(1)

		go func(dep string) {
			defer wg.Done()

			version := cmp.Or(composerDepVersion(ctx, runner, dep), "version unknown")

			mu.Lock()
			installed[dep] = version
			mu.Unlock()
		}(dep)
	}

	wg.Wait()
}

func composerDepVersion(ctx context.Context, runner exec.Runner, dep string) string {
	output, err := runner.Run(ctx, "composer", "show", dep)

	if err != nil {
		return ""
	}

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "versions :") || strings.HasPrefix(line, "version :") {
			parts := strings.SplitN(line, ":", 2)

			if len(parts) <= 1 {
				continue
			}

			return strings.TrimSpace(strings.TrimPrefix(parts[1], "* "))
		}
	}

	return ""
}
