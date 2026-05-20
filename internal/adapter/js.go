package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
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

	var installedPackages map[string]string

	if isPnPProject(deps.FS, deps.Loader.WorkDir) {
		installedPackages = installedFromYarnLock(deps.FS, deps.Loader.WorkDir)
	} else {
		installedPackages = getInstalledPackages(ctx, deps.FS, deps.Loader.WorkDir, packageConfig.Dependencies, packageConfig.DevDependencies, packageConfig.OptionalDependencies)
	}

	for _, dep := range append(packageConfig.Dependencies, packageConfig.DevDependencies...) {
		isDev := slices.Contains(packageConfig.DevDependencies, dep)

		if version, installed := installedPackages[dep]; installed {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed package %s%s (%s)", terminal.Reset, dep, version), Nested: true, Dev: isDev})
		} else {
			errs = append(errs, Message{Text: fmt.Sprintf("Missing package %s%s, Run `%s install %s`", terminal.Reset, dep, packageManager.Command(), dep), Nested: true, Dev: isDev})
		}
	}

	skippedCrossPlatform := 0

	for _, dep := range packageConfig.OptionalDependencies {
		if version, installed := installedPackages[dep]; installed {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed package %s%s (%s)", terminal.Reset, dep, version), Nested: true, Optional: true})
			continue
		}

		if !optionalDepMatchesHost(dep) {
			skippedCrossPlatform++
			continue
		}

		warns = append(warns, Message{Text: fmt.Sprintf("Optional package %s%s not installed", terminal.Reset, dep), Nested: true, Optional: true})
	}

	if skippedCrossPlatform > 0 {
		noun := "packages"

		if skippedCrossPlatform == 1 {
			noun = "package"
		}

		succs = append(succs, Message{
			Text:     fmt.Sprintf("%d %s skipped (platform mismatch)", skippedCrossPlatform, noun),
			Nested:   true,
			Optional: true,
			Info:     true,
		})
	}

	return errs, warns, succs
}

func (p PackageModule) ListOutdated(ctx context.Context, deps Dependencies) ([]OutdatedPackage, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadPackageConfig()

	if !config.HasConfig {
		return nil, nil
	}

	switch config.PackageManager.Command() {
	case "bun", "yarn":
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

	hostOptionals := make([]string, 0, len(config.OptionalDependencies))

	for _, dep := range config.OptionalDependencies {
		if optionalDepMatchesHost(dep) {
			hostOptionals = append(hostOptionals, dep)
		}
	}

	direct := toSet(config.Dependencies, config.DevDependencies, hostOptionals)

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

func getInstalledPackages(ctx context.Context, fsys fs.FS, workDir string, dependencies, devDependencies, optionalDependencies []string) map[string]string {
	installedPackages := make(map[string]string)

	if ctx.Err() != nil {
		return installedPackages
	}

	candidates := make(map[string]struct{}, len(dependencies)+len(devDependencies)+len(optionalDependencies))

	for _, dep := range dependencies {
		candidates[dep] = struct{}{}
	}

	for _, devDep := range devDependencies {
		candidates[devDep] = struct{}{}
	}

	for _, optionalDep := range optionalDependencies {
		candidates[optionalDep] = struct{}{}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for dep := range candidates {
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

			if version == "" {
				return
			}

			mu.Lock()
			installedPackages[dep] = version
			mu.Unlock()
		}(dep)
	}

	wg.Wait()

	return installedPackages
}

func isPnPProject(fsys fs.FS, workDir string) bool {
	for _, marker := range []string{".pnp.cjs", ".pnp.loader.mjs"} {
		if _, err := fsys.Stat(filepath.Join(workDir, marker)); err == nil {
			return true
		}
	}

	return false
}

func isYarnBerry(fsys fs.FS, workDir string) bool {
	_, err := fsys.Stat(filepath.Join(workDir, ".yarnrc.yml"))
	return err == nil
}

func installedFromYarnLock(fsys fs.FS, workDir string) map[string]string {
	data, err := fsys.ReadFile(filepath.Join(workDir, "yarn.lock"))
	if err != nil {
		return map[string]string{}
	}

	parser, ok := lockdiff.ParserFor("yarn.lock")

	if !ok {
		return map[string]string{}
	}

	packages, err := parser.Parse(data)
	if err != nil {
		return map[string]string{}
	}

	return packages
}

var npmOSToGoos = map[string]string{
	"linux":   "linux",
	"darwin":  "darwin",
	"win32":   "windows",
	"freebsd": "freebsd",
	"openbsd": "openbsd",
	"netbsd":  "netbsd",
	"android": "android",
	"sunos":   "solaris",
}

var npmArchToGoarch = map[string]string{
	"x64":     "amd64",
	"ia32":    "386",
	"arm":     "arm",
	"arm64":   "arm64",
	"mips":    "mips",
	"mipsel":  "mipsle",
	"ppc64":   "ppc64",
	"ppc64le": "ppc64le",
	"riscv64": "riscv64",
	"s390x":   "s390x",
}

func optionalDepMatchesHost(name string) bool {
	return optionalDepMatchesPlatform(name, runtime.GOOS, runtime.GOARCH)
}

func optionalDepMatchesPlatform(name, goos, goarch string) bool {
	tokens := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '/' || r == '@'
	})

	for _, token := range tokens {
		if mappedOS, ok := npmOSToGoos[token]; ok && mappedOS != goos {
			return false
		}

		if mappedArch, ok := npmArchToGoarch[token]; ok && mappedArch != goarch {
			return false
		}
	}

	return true
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
