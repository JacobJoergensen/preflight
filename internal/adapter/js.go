package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/manifest"
	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func init() {
	Register(PackageModule{})
}

type PackageModule struct{}

func (p PackageModule) Name() string {
	return "js"
}

func (p PackageModule) DisplayName() string {
	return "JavaScript"
}

func (p PackageModule) Check(ctx context.Context, deps Dependencies) ([]Message, []Message, []Message) {
	errs := []Message{}
	warns := []Message{}
	succs := []Message{}

	if ctx.Err() != nil {
		return errs, warns, succs
	}

	packageConfig := deps.Loader.LoadPackageConfig()
	packageManager := packageConfig.PackageManager

	if !packageConfig.HasConfig {
		skip, extra := warnsWhenNoPackageJSON(deps, packageManager)

		if skip {
			return errs, warns, succs
		}

		warns = append(warns, extra...)
		return errs, warns, succs
	}

	if packageConfig.Error != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Failed to read package.json: %v", packageConfig.Error)})
		return errs, warns, succs
	}

	type engineRequirement struct {
		command         string
		requiredVersion string
	}

	engineRequirements := []engineRequirement{
		{command: "node", requiredVersion: packageConfig.NodeVersion},
		{command: packageManager.Command(), requiredVersion: requiredPMVersion(packageConfig, packageManager.Command())},
	}

	for _, requirement := range engineRequirements {
		if requirement.requiredVersion == "" {
			continue
		}

		installedVersion, err := deps.Runner.Run(ctx, requirement.command, "--version")

		if err != nil {
			warns = append(warns, Message{Text: fmt.Sprintf("Could not retrieve version for '%s': %v", requirement.command, err)})
			continue
		}

		var valid bool

		if requirement.command == "node" {
			valid = nodeEngineSatisfiedByRuntime(normalizeNodeVersionOutput(installedVersion), requirement.requiredVersion)
		} else {
			valid, _ = semver.ValidateVersion(installedVersion, requirement.requiredVersion)
		}

		if !valid {
			warns = append(warns, Message{Text: fmt.Sprintf("Missing %s%s (%s ⟶ required %s)", terminal.Reset, requirement.command, installedVersion, requirement.requiredVersion)})
		}
	}

	if packageConfig.NodeVersion != "" {
		output, err := deps.Runner.Run(ctx, "node", "--version")

		if err == nil {
			installedVersion := normalizeNodeVersionOutput(output)

			if nodeEngineSatisfiedByRuntime(installedVersion, packageConfig.NodeVersion) {
				succs = append(succs, Message{Text: fmt.Sprintf("Installed %snode (%s ⟶ required %s)", terminal.Reset, installedVersion, packageConfig.NodeVersion)})
			}
		}
	}

	packageTool, ok := manifest.GetTool(packageManager.Command())

	if ok {
		version, err := deps.Runner.Run(ctx, packageTool.Command, packageTool.VersionArgs...)

		if err == nil {
			trimmedVersion := trimFirstLine(version)
			requirement := requiredPMVersion(packageConfig, packageManager.Command())

			if requirement != "" {
				if ok, _ := semver.ValidateVersion(trimmedVersion, requirement); ok {
					succs = append(succs, Message{Text: fmt.Sprintf("Installed %s%s (%s ⟶ required %s)", terminal.Reset, packageTool.Name, trimmedVersion, requirement)})
				}
			} else {
				succs = append(succs, Message{Text: fmt.Sprintf("Installed %s%s (%s)", terminal.Reset, packageTool.Name, trimmedVersion)})
			}
		}
	}

	succs = append(succs, Message{Text: "package.json found:"})
	installedPackages := getInstalledPackages(ctx, deps.FS, deps.Loader.WorkDir, packageConfig.Dependencies, packageConfig.DevDependencies)

	for _, dep := range append(packageConfig.Dependencies, packageConfig.DevDependencies...) {
		isDev := slices.Contains(packageConfig.DevDependencies, dep)

		if version, installed := installedPackages[dep]; installed {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed package %s%s (%s)", terminal.Reset, dep, version), Nested: true, Dev: isDev})
		} else {
			errs = append(errs, Message{Text: fmt.Sprintf("Missing package %s%s, Run `%s install %s`", terminal.Reset, dep, packageManager.Command(), dep), Nested: true, Dev: isDev})
		}
	}

	return errs, warns, succs
}

func (p PackageModule) ListDependencies(ctx context.Context, deps Dependencies) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadPackageConfig()

	if !config.HasConfig || config.Error != nil {
		return nil, config.Error
	}

	return append(slices.Clone(config.Dependencies), config.DevDependencies...), nil
}

func (p PackageModule) ListOutdated(ctx context.Context, deps Dependencies) ([]OutdatedPackage, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadPackageConfig()

	if !config.HasConfig {
		return nil, nil
	}

	output, err := deps.Runner.Run(ctx, config.PackageManager.Command(), "outdated", "--json")

	if err != nil && output == "" {
		return nil, err
	}

	packages, err := parseNPMOutdated(output)

	if err != nil {
		return nil, err
	}

	direct := toSet(config.Dependencies, config.DevDependencies)

	return filterDirectOutdated(packages, direct), nil
}

func parseNPMOutdated(output string) ([]OutdatedPackage, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	var data map[string]struct {
		Current string `json:"current"`
		Latest  string `json:"latest"`
	}

	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, err
	}

	packages := make([]OutdatedPackage, 0, len(data))

	for name, info := range data {
		packages = append(packages, OutdatedPackage{
			Name:    name,
			Current: info.Current,
			Latest:  info.Latest,
		})
	}

	slices.SortFunc(packages, func(a, b OutdatedPackage) int {
		return strings.Compare(a.Name, b.Name)
	})

	return packages, nil
}

func (p PackageModule) Fix(ctx context.Context, deps Dependencies, selectors []string, options FixOptions) (FixItem, error) {
	return fixWithSelectorCheck(ctx, deps, p.Name(), manifest.PackageTypeJS, selectors, options)
}

func warnsWhenNoPackageJSON(deps Dependencies, packageManager manifest.PackageManager) (skip bool, warns []Message) {
	fileInfo, err := deps.FS.Stat(filepath.Join(deps.Loader.WorkDir, "node_modules"))

	if err != nil || !fileInfo.IsDir() {
		return true, nil
	}

	warns = []Message{{Text: "package.json not found."}}

	if packageManager.LockFile() != "" {
		warns = append(warns, Message{Text: fmt.Sprintf(
			"package.json not found, but %s exists. Ensure package.json is included in your project.",
			packageManager.LockFile(),
		)})
	} else {
		warns = append(warns, Message{Text: "Neither package.json nor lock files (package-lock.json, bun.lock, pnpm-lock.yaml or yarn.lock) were found."})
	}

	return false, warns
}

func requiredPMVersion(config manifest.PackageConfig, command string) string {
	switch command {
	case "npm":
		return config.NPMVersion
	case "pnpm":
		return config.PNPMVersion
	case "yarn":
		return config.YarnVersion
	case "bun":
		return config.BunVersion
	default:
		return ""
	}
}

func getInstalledPackages(ctx context.Context, fsys fs.FS, workDir string, dependencies, devDependencies []string) map[string]string {
	installedPackages := make(map[string]string)

	if ctx.Err() != nil {
		return installedPackages
	}

	for _, dep := range dependencies {
		installedPackages[dep] = "unknown"
	}

	for _, devDep := range devDependencies {
		installedPackages[devDep] = "unknown"
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for dep := range installedPackages {
		wg.Add(1)

		go func(dep string) {
			defer wg.Done()

			if ctx.Err() != nil {
				return
			}

			path, valid := buildPackagePath(dep)

			if !valid {
				return
			}

			version := readPackageVersion(fsys, filepath.Join(workDir, path))

			if version != "" {
				mu.Lock()
				installedPackages[dep] = version
				mu.Unlock()
			}
		}(dep)
	}

	wg.Wait()

	return installedPackages
}

func buildPackagePath(name string) (string, bool) {
	if strings.Contains(name, "..") || (strings.Contains(name, "/") && !strings.HasPrefix(name, "@")) {
		return "", false
	}

	var path string

	if strings.HasPrefix(name, "@") {
		parts := strings.SplitN(name, "/", 2)

		if len(parts) != 2 || strings.Contains(parts[1], "..") || strings.Contains(parts[1], "/") {
			return "", false
		}

		path = filepath.Join("node_modules", parts[0], parts[1], "package.json")
	} else {
		path = filepath.Join("node_modules", name, "package.json")
	}

	path = filepath.Clean(path)

	if !strings.HasPrefix(path, filepath.Join("node_modules", "")) {
		return "", false
	}

	return path, true
}

func readPackageVersion(fsys fs.FS, path string) string {
	data, err := fsys.ReadFile(path)

	if err != nil {
		return ""
	}

	var packageInfo struct {
		Version string `json:"version"`
	}

	if json.Unmarshal(data, &packageInfo) != nil {
		return ""
	}

	return packageInfo.Version
}

func normalizeNodeVersionOutput(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")

	return version
}
