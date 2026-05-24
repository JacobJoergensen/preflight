package js

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/parallel"
	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

var managers = []ecosystem.Manager{
	{
		Command: "bun", DisplayName: "Bun", ConfigFile: "package.json", LockFile: "bun.lock",
		VersionArgs: []string{"--version"}, InstallArgs: []string{"install"}, ForceArgs: []string{"--force"},
		Audit: &ecosystem.AuditProbe{Args: []string{"audit", "--json"}, Parse: parseNPMVulnerabilityFindings},
	},
	{
		Command: "pnpm", DisplayName: "PNPM", ConfigFile: "package.json", LockFile: "pnpm-lock.yaml",
		VersionArgs: []string{"--version"}, InstallArgs: []string{"install"}, ForceArgs: []string{"--force"},
		Outdated: &ecosystem.OutdatedProbe{Args: []string{"outdated", "--json"}, Parse: parseOutdated},
		Audit:    &ecosystem.AuditProbe{Args: []string{"audit", "--json"}, Parse: parseNPMVulnerabilityFindings},
	},
	{
		Command: "yarn", DisplayName: "Yarn", ConfigFile: "package.json", LockFile: "yarn.lock",
		VersionArgs: []string{"--version"}, InstallArgs: []string{"install"}, ForceArgs: []string{"--force"},
		Audit: &ecosystem.AuditProbe{Args: []string{"npm", "audit", "--json"}, Parse: parseYarnNpmAuditFindings},
	},
	{
		Command: "npm", DisplayName: "NPM", ConfigFile: "package.json", LockFile: "package-lock.json",
		VersionArgs: []string{"--version"}, InstallArgs: []string{"install"}, ForceArgs: []string{"--force"},
		Outdated: &ecosystem.OutdatedProbe{Args: []string{"outdated", "--json"}, Parse: parseOutdated},
		Audit:    &ecosystem.AuditProbe{Args: []string{"audit", "--json"}, Parse: parseNPMVulnerabilityFindings},
	},
}

var detectMarkers = []ecosystem.Marker{
	{File: "package.json", Manager: "npm"},
	{File: "node_modules", Manager: "npm"},
}

func Spec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:            "js",
		DisplayName:     "JavaScript",
		Priority:        4,
		Managers:        managers,
		RuntimeCommands: []string{"node"},
		Detect:          detectMarkers,
		Check:           check,
		License:         scanLicenses,
		ExtraSignals:    projectSignals,
	}
}

func scanLicenses(_ context.Context, rc ecosystem.RunContext, _ ecosystem.Detection) ecosystem.LicenseResult {
	root := filepath.Join(rc.WorkDir, "node_modules")

	entries, err := rc.FS.ReadDir(root)
	if err != nil {
		return ecosystem.LicenseResult{Skipped: true, SkipReason: "node_modules not found (run install first)"}
	}

	var packages []ecosystem.PackageLicense

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		if strings.HasPrefix(name, ".") {
			continue
		}

		if strings.HasPrefix(name, "@") {
			packages = append(packages, scopedModuleLicenses(rc.FS, filepath.Join(root, name))...)
			continue
		}

		if pkg, ok := readModuleLicense(rc.FS, filepath.Join(root, name)); ok {
			packages = append(packages, pkg)
		}
	}

	ecosystem.SortPackageLicenses(packages)

	return ecosystem.LicenseResult{Packages: packages}
}

func scopedModuleLicenses(fsys fs.FS, scopeDir string) []ecosystem.PackageLicense {
	entries, err := fsys.ReadDir(scopeDir)
	if err != nil {
		return nil
	}

	var packages []ecosystem.PackageLicense

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if pkg, ok := readModuleLicense(fsys, filepath.Join(scopeDir, entry.Name())); ok {
			packages = append(packages, pkg)
		}
	}

	return packages
}

func readModuleLicense(fsys fs.FS, dir string) (ecosystem.PackageLicense, bool) {
	raw, err := fsys.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ecosystem.PackageLicense{}, false
	}

	var manifest struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		License  string `json:"license"`
		Licenses []struct {
			Type string `json:"type"`
		} `json:"licenses"`
	}

	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.Name == "" {
		return ecosystem.PackageLicense{}, false
	}

	license := manifest.License

	if license == "" && len(manifest.Licenses) > 0 {
		types := make([]string, 0, len(manifest.Licenses))

		for _, entry := range manifest.Licenses {
			if entry.Type != "" {
				types = append(types, entry.Type)
			}
		}

		license = strings.Join(types, " OR ")
	}

	return ecosystem.PackageLicense{Name: manifest.Name, Version: manifest.Version, License: license}, true
}

func projectSignals(rc ecosystem.RunContext) []string {
	lockToTool := []struct {
		lockFile string
		tool     string
	}{
		{"package-lock.json", "npm"},
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lock", "bun"},
	}

	tools := make(map[string]struct{})

	for _, entry := range lockToTool {
		if rc.FileExists(entry.lockFile) {
			tools[entry.tool] = struct{}{}
		}
	}

	var lines []string

	if rc.FileExists("package.json") {
		if raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "package.json")); err == nil && workspacesConfigured(raw) {
			lines = append(lines, "package.json workspaces configured")
		}
	}

	if rc.FileExists("pnpm-workspace.yaml") {
		lines = append(lines, "pnpm workspace configured")
	}

	if len(tools) > 1 {
		lines = append(lines, "Note: lockfiles from more than one JS package manager — pick one tool and remove stray lockfiles when you can.")
	}

	return lines
}

func workspacesConfigured(raw []byte) bool {
	var probe struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}

	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}

	trimmed := strings.TrimSpace(string(probe.Workspaces))

	if trimmed == "" || trimmed == "null" {
		return false
	}

	var arr []any

	if json.Unmarshal(probe.Workspaces, &arr) == nil {
		return len(arr) > 0
	}

	var obj map[string]any

	if json.Unmarshal(probe.Workspaces, &obj) == nil {
		return len(obj) > 0
	}

	return true
}

type jsConfig struct {
	NodeVersion          string
	NPMVersion           string
	PNPMVersion          string
	YarnVersion          string
	BunVersion           string
	Dependencies         []string
	DevDependencies      []string
	OptionalDependencies []string
}

type depVersion struct {
	name    string
	version string
}

func check(ctx context.Context, rc ecosystem.RunContext, detection ecosystem.Detection) []model.Message {
	if ctx.Err() != nil {
		return nil
	}

	if !rc.FileExists("package.json") {
		return warnsWhenNoPackageJSON(detection.Active.LockFile)
	}

	config, err := loadConfig(rc)
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Failed to read package.json: %v", err)}}
	}

	manager := detection.Active

	var messages []model.Message

	requirements := []struct {
		command         string
		requiredVersion string
	}{
		{"node", config.NodeVersion},
		{manager.Command, requiredPMVersion(config, manager.Command)},
	}

	for _, requirement := range requirements {
		if requirement.requiredVersion == "" {
			continue
		}

		result, err := rc.Runner.Run(ctx, requirement.command, "--version")
		if err != nil {
			messages = append(messages, model.Message{Severity: model.SeverityWarning, Text: fmt.Sprintf("Could not retrieve version for '%s': %v", requirement.command, err)})
			continue
		}

		var valid bool

		if requirement.command == "node" {
			valid = nodeEngineSatisfiedByRuntime(normalizeNodeVersionOutput(result.Stdout), requirement.requiredVersion)
		} else {
			valid, _ = semver.ValidateVersion(result.Stdout, requirement.requiredVersion)
		}

		if !valid {
			messages = append(
				messages,
				model.Message{Severity: model.SeverityWarning, Text: fmt.Sprintf("Missing %s%s (%s ⟶ required %s)", terminal.Reset, requirement.command, result.Stdout, requirement.requiredVersion)},
			)
		}
	}

	if config.NodeVersion != "" {
		result, err := rc.Runner.Run(ctx, "node", "--version")

		if err == nil {
			installedVersion := normalizeNodeVersionOutput(result.Stdout)

			if nodeEngineSatisfiedByRuntime(installedVersion, config.NodeVersion) {
				messages = append(
					messages,
					model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %snode (%s ⟶ required %s)", terminal.Reset, installedVersion, config.NodeVersion)},
				)
			}
		}
	}

	if result, err := rc.Runner.Run(ctx, manager.Command, manager.VersionArgs...); err == nil {
		trimmedVersion := ecosystem.FirstLine(result.Stdout)
		requirement := requiredPMVersion(config, manager.Command)

		switch requirement {
		case "":
			messages = append(messages, model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %s%s (%s)", terminal.Reset, manager.DisplayName, trimmedVersion)})
		default:
			if ok, _ := semver.ValidateVersion(trimmedVersion, requirement); ok {
				messages = append(
					messages,
					model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %s%s (%s ⟶ required %s)", terminal.Reset, manager.DisplayName, trimmedVersion, requirement)},
				)
			}
		}
	}

	hasDependencies := len(config.Dependencies)+len(config.DevDependencies)+len(config.OptionalDependencies) > 0
	messages = append(messages, ecosystem.MissingLockfileWarning(rc, manager, hasDependencies)...)

	var installed map[string]string

	if isPnPProject(rc) {
		installed = installedFromYarnLock(rc)
	} else {
		installed = getInstalledPackages(ctx, rc.FS, rc.WorkDir, config.Dependencies, config.DevDependencies, config.OptionalDependencies)
	}

	for _, dep := range slices.Concat(config.Dependencies, config.DevDependencies) {
		isDev := slices.Contains(config.DevDependencies, dep)

		if version, ok := installed[dep]; ok {
			messages = append(messages, model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed package %s%s (%s)", terminal.Reset, dep, version), Nested: true, Dev: isDev})
		} else {
			messages = append(
				messages,
				model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("Missing package %s%s, Run `%s install %s`", terminal.Reset, dep, manager.Command, dep), Nested: true, Dev: isDev},
			)
		}
	}

	skippedCrossPlatform := 0

	for _, dep := range config.OptionalDependencies {
		if version, ok := installed[dep]; ok {
			messages = append(messages, model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed package %s%s (%s)", terminal.Reset, dep, version), Nested: true, Optional: true})
			continue
		}

		if !optionalDepMatchesHost(dep) {
			skippedCrossPlatform++
			continue
		}

		messages = append(messages, model.Message{Severity: model.SeverityWarning, Text: fmt.Sprintf("Optional package %s%s not installed", terminal.Reset, dep), Nested: true, Optional: true})
	}

	if skippedCrossPlatform > 0 {
		noun := "packages"

		if skippedCrossPlatform == 1 {
			noun = "package"
		}

		messages = append(
			messages,
			model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("%d %s skipped (platform mismatch)", skippedCrossPlatform, noun), Nested: true, Optional: true, Info: true},
		)
	}

	return messages
}

func warnsWhenNoPackageJSON(lockFile string) []model.Message {
	warnings := []model.Message{{Severity: model.SeverityWarning, Text: "package.json not found."}}

	if lockFile != "" {
		warnings = append(warnings, model.Message{
			Severity: model.SeverityWarning,
			Text:     fmt.Sprintf("package.json not found, but %s exists. Ensure package.json is included in your project.", lockFile),
		})
	} else {
		warnings = append(warnings, model.Message{
			Severity: model.SeverityWarning,
			Text:     "Neither package.json nor lock files (package-lock.json, bun.lock, pnpm-lock.yaml or yarn.lock) were found.",
		})
	}

	return warnings
}

func parseOutdated(rc ecosystem.RunContext, output string) ([]ecosystem.OutdatedPackage, error) {
	packages, err := parseNPMOutdated(output)
	if err != nil {
		return nil, err
	}

	config, err := loadConfig(rc)
	if err != nil {
		return nil, nil
	}

	hostOptionals := make([]string, 0, len(config.OptionalDependencies))

	for _, dep := range config.OptionalDependencies {
		if optionalDepMatchesHost(dep) {
			hostOptionals = append(hostOptionals, dep)
		}
	}

	direct := ecosystem.ToSet(config.Dependencies, config.DevDependencies, hostOptionals)

	return ecosystem.FilterDirect(packages, direct), nil
}

func parseNPMOutdated(output string) ([]ecosystem.OutdatedPackage, error) {
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

	packages := make([]ecosystem.OutdatedPackage, 0, len(data))

	for name, info := range data {
		packages = append(packages, ecosystem.OutdatedPackage{
			Name:    name,
			Current: info.Current,
			Latest:  info.Latest,
		})
	}

	ecosystem.SortOutdated(packages)

	return packages, nil
}

func loadConfig(rc ecosystem.RunContext) (jsConfig, error) {
	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "package.json"))
	if err != nil {
		return jsConfig{}, err
	}

	return parsePackageJSON(raw)
}

func parsePackageJSON(raw []byte) (jsConfig, error) {
	var data struct {
		Engines struct {
			Node string `json:"node"`
			NPM  string `json:"npm,omitempty"`
			PNPM string `json:"pnpm,omitempty"`
			Yarn string `json:"yarn,omitempty"`
			Bun  string `json:"bun,omitempty"`
		} `json:"engines"`
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}

	if err := json.Unmarshal(raw, &data); err != nil {
		return jsConfig{}, err
	}

	return jsConfig{
		NodeVersion:          strings.TrimSpace(data.Engines.Node),
		NPMVersion:           strings.TrimSpace(data.Engines.NPM),
		PNPMVersion:          strings.TrimSpace(data.Engines.PNPM),
		YarnVersion:          strings.TrimSpace(data.Engines.Yarn),
		BunVersion:           strings.TrimSpace(data.Engines.Bun),
		Dependencies:         slices.Sorted(maps.Keys(data.Dependencies)),
		DevDependencies:      slices.Sorted(maps.Keys(data.DevDependencies)),
		OptionalDependencies: slices.Sorted(maps.Keys(data.OptionalDependencies)),
	}, nil
}

func requiredPMVersion(config jsConfig, command string) string {
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
	installed := make(map[string]string)

	if ctx.Err() != nil {
		return installed
	}

	candidates := make(map[string]struct{}, len(dependencies)+len(devDependencies)+len(optionalDependencies))

	for _, dep := range slices.Concat(dependencies, devDependencies, optionalDependencies) {
		candidates[dep] = struct{}{}
	}

	candidateList := make([]string, 0, len(candidates))

	for dep := range candidates {
		candidateList = append(candidateList, dep)
	}

	found := parallel.Collect(ctx, candidateList, func(_ context.Context, dep string) (depVersion, bool) {
		path, valid := buildPackagePath(dep)

		if !valid {
			return depVersion{}, false
		}

		version := readPackageVersion(fsys, filepath.Join(workDir, path))

		if version == "" {
			return depVersion{}, false
		}

		return depVersion{name: dep, version: version}, true
	})

	for _, pkg := range found {
		installed[pkg.name] = pkg.version
	}

	return installed
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

func isPnPProject(rc ecosystem.RunContext) bool {
	return slices.ContainsFunc([]string{".pnp.cjs", ".pnp.loader.mjs"}, rc.FileExists)
}

func installedFromYarnLock(rc ecosystem.RunContext) map[string]string {
	data, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "yarn.lock"))
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

func normalizeNodeVersionOutput(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func parseYarnNpmAuditFindings(jsonText string) []model.Finding {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "{") {
		return nil
	}

	var advisories map[string][]struct {
		Severity   string   `json:"severity"`
		ModuleName string   `json:"module_name"`
		Title      string   `json:"title"`
		URL        string   `json:"url"`
		CVEs       []string `json:"cves"`
	}

	if err := json.Unmarshal([]byte(jsonText), &advisories); err != nil {
		return nil
	}

	var findings []model.Finding

	for key, list := range advisories {
		for _, advisory := range list {
			if strings.TrimSpace(advisory.Severity) == "" {
				continue
			}

			pkg := advisory.ModuleName
			if pkg == "" {
				pkg = key
			}

			id := ""
			var aliases []string

			if len(advisory.CVEs) > 0 {
				id = advisory.CVEs[0]
				aliases = advisory.CVEs[1:]
			}

			findings = append(findings, model.Finding{
				ID:       id,
				Aliases:  aliases,
				Severity: ecosystem.NormalizeSeverity(advisory.Severity),
				Package:  pkg,
				URL:      advisory.URL,
				Summary:  advisory.Title,
			})
		}
	}

	ecosystem.SortFindings(findings)

	return findings
}

func parseNPMVulnerabilityFindings(jsonText string) []model.Finding {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "{") {
		return nil
	}

	var root struct {
		Vulnerabilities map[string]struct {
			Severity string            `json:"severity"`
			Via      []json.RawMessage `json:"via"`
		} `json:"vulnerabilities"`
	}

	if err := json.Unmarshal([]byte(jsonText), &root); err != nil {
		return nil
	}

	var findings []model.Finding

	seen := make(map[string]struct{})

	for name, entry := range root.Vulnerabilities {
		for _, raw := range entry.Via {
			var via struct {
				Name     string `json:"name"`
				Title    string `json:"title"`
				URL      string `json:"url"`
				Severity string `json:"severity"`
			}

			// A string `via` entry references a transitive package rather than an
			// advisory; unmarshaling it into the struct fails, so we skip it.
			if err := json.Unmarshal(raw, &via); err != nil {
				continue
			}

			severity := via.Severity
			if severity == "" {
				severity = entry.Severity
			}

			pkg := via.Name
			if pkg == "" {
				pkg = name
			}

			id := ghsaFromURL(via.URL)

			key := id + "|" + pkg
			if id == "" {
				key = via.Title + "|" + pkg
			}

			if _, dup := seen[key]; dup {
				continue
			}

			seen[key] = struct{}{}

			findings = append(findings, model.Finding{
				ID:       id,
				Severity: ecosystem.NormalizeSeverity(severity),
				Package:  pkg,
				URL:      via.URL,
				Summary:  via.Title,
			})
		}
	}

	ecosystem.SortFindings(findings)

	return findings
}

func ghsaFromURL(advisoryURL string) string {
	const marker = "/advisories/"

	if i := strings.LastIndex(advisoryURL, marker); i >= 0 {
		return advisoryURL[i+len(marker):]
	}

	return ""
}
