package python

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

var pipAudit = &ecosystem.AuditProbe{
	Tool:            "pip-audit",
	Args:            []string{"--format", "json"},
	ToolMissingHint: "pip-audit not found on PATH (install: pip install pip-audit)",
	Parse:           parsePipAuditFindings,
}

var managers = []ecosystem.Manager{
	{
		Command: "pip", DisplayName: "pip", ConfigFile: "requirements.txt",
		VersionArgs: []string{"--version"}, InstallArgs: []string{"install", "-r", "requirements.txt"}, ForceArgs: []string{"--upgrade"},
		Outdated: &ecosystem.OutdatedProbe{Tool: "python", Args: []string{"-m", "pip", "list", "--outdated", "--format=json"}, Parse: outdatedParser("pip", parsePipOutdated)},
		Audit:    pipAudit,
	},
	{
		Command: "poetry", DisplayName: "Poetry", ConfigFile: "pyproject.toml", LockFile: "poetry.lock",
		VersionArgs: []string{"--version"}, InstallArgs: []string{"install"}, ForceArgs: []string{"--sync"},
		Outdated: &ecosystem.OutdatedProbe{Tool: "poetry", Args: []string{"show", "--outdated", "--format", "json"}, Parse: outdatedParser("poetry", parsePipOutdated)},
		Audit:    pipAudit,
	},
	{
		Command: "uv", DisplayName: "uv", ConfigFile: "pyproject.toml", LockFile: "uv.lock",
		VersionArgs: []string{"--version"}, InstallArgs: []string{"sync"}, ForceArgs: []string{"--frozen"},
		Outdated: &ecosystem.OutdatedProbe{Tool: "uv", Args: []string{"run", "python", "-m", "pip", "list", "--outdated", "--format=json"}, Parse: outdatedParser("uv", parsePipOutdated)},
		Audit:    &ecosystem.AuditProbe{Args: []string{"audit", "--format", "json"}, Parse: parseUvAuditFindings},
	},
	{
		Command: "pipenv", DisplayName: "Pipenv", ConfigFile: "Pipfile", LockFile: "Pipfile.lock",
		VersionArgs: []string{"--version"}, InstallArgs: []string{"install"},
		Outdated: &ecosystem.OutdatedProbe{Tool: "pipenv", Args: []string{"run", "python", "-m", "pip", "list", "--outdated", "--format=json"}, Parse: outdatedParser("pipenv", parsePipOutdated)},
		Audit:    pipAudit,
	},
	{
		Command: "pdm", DisplayName: "PDM", ConfigFile: "pyproject.toml", LockFile: "pdm.lock",
		VersionArgs: []string{"--version"}, InstallArgs: []string{"install"},
		Outdated: &ecosystem.OutdatedProbe{Tool: "pdm", Args: []string{"outdated", "--json"}, Parse: outdatedParser("pdm", parsePDMOutdated)},
		Audit:    pipAudit,
	},
}

var (
	pythonSemverFromVersion = regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	pep508NameFromLine      = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?)`)
	pep621DevExtraNames     = map[string]struct{}{"dev": {}, "test": {}, "docs": {}}
)

var detectMarkers = []ecosystem.Marker{
	{File: "poetry.lock", Manager: "poetry"},
	{File: "uv.lock", Manager: "uv"},
	{File: "Pipfile.lock", Manager: "pipenv"},
	{File: "pdm.lock", Manager: "pdm"},
	{File: "requirements.txt", Manager: "pip"},
	{File: "pyproject.toml", Contains: "[tool.poetry]", Manager: "poetry"},
	{File: "pyproject.toml", Contains: "[tool.pdm]", Manager: "pdm"},
	{File: "pyproject.toml", Contains: "[tool.uv]", Manager: "uv"},
	{File: "pyproject.toml", Contains: "[project]", Manager: "uv"},
	{File: "Pipfile", Manager: "pipenv"},
}

func Spec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:            "python",
		DisplayName:     "Python",
		Priority:        7,
		Managers:        managers,
		RuntimeCommands: []string{"python", "python3"},
		Detect:          detectMarkers,
		Check:           check,
		License:         scanLicenses,
		VersionPins:     []string{".python-version"},
		EnvSignals:      []string{"VIRTUAL_ENV", "CONDA_PREFIX"},
	}
}

func scanLicenses(ctx context.Context, rc ecosystem.RunContext, _ ecosystem.Detection) ecosystem.LicenseResult {
	return ecosystem.RunLicenseCommand(ctx, rc, "pip-licenses", "pip-licenses not found on PATH (install: pip install pip-licenses)", parsePipLicenses, "--format=json")
}

func parsePipLicenses(jsonText string) []ecosystem.PackageLicense {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "[") {
		return nil
	}

	var entries []struct {
		Name     string   `json:"Name"`
		Version  string   `json:"Version"`
		License  string   `json:"License"`
		Licenses []string `json:"Licenses"`
	}

	if err := json.Unmarshal([]byte(jsonText), &entries); err != nil {
		return nil
	}

	packages := make([]ecosystem.PackageLicense, 0, len(entries))

	for _, entry := range entries {
		license := entry.License

		if license == "" && len(entry.Licenses) > 0 {
			license = strings.Join(entry.Licenses, " OR ")
		}

		packages = append(packages, ecosystem.PackageLicense{
			Name:    entry.Name,
			Version: entry.Version,
			License: license,
		})
	}

	ecosystem.SortPackageLicenses(packages)

	return packages
}

func check(ctx context.Context, rc ecosystem.RunContext, detection ecosystem.Detection) []model.Message {
	if ctx.Err() != nil {
		return nil
	}

	manager := detection.Active
	config := loadConfig(rc, manager.Command)

	if config.Error != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Failed to read Python project files: %v", config.Error)}}
	}

	var messages []model.Message

	if config.PythonVersionPin != "" && config.RequiresPythonConstraint != "" {
		pin := strings.TrimPrefix(strings.TrimSpace(config.PythonVersionPin), "v")

		if !semver.MatchVersionConstraint(pin, config.RequiresPythonConstraint) {
			messages = append(messages, model.Message{Severity: model.SeverityWarning, Text: fmt.Sprintf(
				".python-version pins %s but requires-python is %s — align these for consistent installs.",
				strings.TrimSpace(config.PythonVersionPin), config.RequiresPythonConstraint,
			)})
		}
	}

	if config.RequiresPython != "" {
		pythonVersion, err := pythonRuntimeVersion(ctx, rc)
		if err != nil {
			messages = append(messages, model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("Python is not installed or not on PATH: %v", err)})
			return messages
		}

		if ok, msg := semver.ValidateVersion(pythonVersion, config.RequiresPython); !ok {
			if msg != "" {
				messages = append(messages, model.Message{Severity: model.SeverityWarning, Text: msg})
			} else {
				messages = append(
					messages,
					model.Message{Severity: model.SeverityWarning, Text: fmt.Sprintf("Python version %s does not satisfy requires-python %s", pythonVersion, config.RequiresPython)},
				)
			}
		}
	}

	installedVersion, err := ecosystem.ToolVersion(ctx, rc, manager)
	if err != nil {
		messages = append(messages, model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("%s is not installed or not on PATH: %v", manager.DisplayName, err)})
		return messages
	}

	version := ecosystem.FirstLine(installedVersion)
	rhs := ""

	switch {
	case manager.LockFile != "" && rc.FileExists(manager.LockFile):
		rhs = manager.LockFile
	case manager.ConfigFile != "" && rc.FileExists(manager.ConfigFile):
		rhs = manager.ConfigFile
	}

	if rhs != "" {
		messages = append(messages, model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %s%s (%s ⟶ %s)", terminal.Reset, manager.DisplayName, version, rhs)})
	} else {
		messages = append(messages, model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %s%s (%s)", terminal.Reset, manager.DisplayName, version)})
	}

	hasDependencies := len(config.Dependencies)+len(config.DevDependencies)+len(config.OptionalDependencies) > 0
	messages = append(messages, ecosystem.MissingLockfileWarning(rc, manager, hasDependencies)...)

	if err := runPipCheck(ctx, rc, manager.Command); err != nil {
		messages = append(messages, model.Message{Severity: model.SeverityError, Text: ecosystem.FormatExecFailure("pip check failed", err)})
	}

	installed := installedPackages(ctx, rc, manager.Command)

	seen := make(map[string]struct{})

	process := func(dep string, isDev bool) {
		if dep == "" {
			return
		}

		key := strings.ToLower(dep)

		if _, dup := seen[key]; dup {
			return
		}

		seen[key] = struct{}{}

		if installedVersion, ok := installed[key]; ok {
			messages = append(
				messages,
				model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed package %s%s (%s)", terminal.Reset, dep, installedVersion), Nested: true, Dev: isDev},
			)
		} else {
			messages = append(
				messages,
				model.Message{
					Severity: model.SeverityError,
					Text:     fmt.Sprintf("Missing package %s%s, Run your install command (e.g. `%s install`).", terminal.Reset, dep, manager.Command),
					Nested:   true,
					Dev:      isDev,
				},
			)
		}
	}

	for _, dep := range config.Dependencies {
		process(dep, false)
	}

	for _, dep := range config.DevDependencies {
		process(dep, true)
	}

	return messages
}

type pythonConfig struct {
	Dependencies             []string
	DevDependencies          []string
	OptionalDependencies     []string
	RequiresPython           string // effective constraint (pyproject or .python-version fallback)
	RequiresPythonConstraint string // from pyproject.toml only
	PythonVersionPin         string // raw .python-version content
	Error                    error
}

func loadConfig(rc ecosystem.RunContext, command string) pythonConfig {
	var config pythonConfig

	var err error

	switch command {
	case "pip":
		config.Dependencies, err = loadRequirementsTxtDeps(rc, "requirements.txt")
	case "poetry":
		config.Dependencies, config.DevDependencies, config.OptionalDependencies, config.RequiresPythonConstraint, err = loadPoetryPyproject(rc)
	case "uv", "pdm":
		config.Dependencies, config.DevDependencies, config.OptionalDependencies, config.RequiresPythonConstraint, err = loadPEP621Pyproject(rc)
	case "pipenv":
		config.Dependencies, config.DevDependencies, err = loadPipfileDeps(rc)
	default:
		err = fmt.Errorf("unsupported python tool: %s", command)
	}

	if err != nil {
		config.Error = err
	}

	config.PythonVersionPin = ecosystem.ReadVersionPin(rc, ".python-version")
	config.RequiresPython = config.RequiresPythonConstraint

	if config.RequiresPython == "" && config.PythonVersionPin != "" {
		config.RequiresPython = config.PythonVersionPin
	}

	return config
}

func loadRequirementsTxtDeps(rc ecosystem.RunContext, filename string) ([]string, error) {
	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, filename))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	return parseRequirementsTxt(string(raw)), nil
}

func parseRequirementsTxt(content string) []string {
	var names []string

	seen := make(map[string]struct{})

	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])

		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}

		name := extractPackageName(line)

		if name == "" || strings.ContainsAny(name, " \t") {
			continue
		}

		base := strings.Split(name, "[")[0]

		if base == "" {
			continue
		}

		lower := strings.ToLower(base)

		if _, dup := seen[lower]; dup {
			continue
		}

		seen[lower] = struct{}{}
		names = append(names, base)
	}

	slices.Sort(names)

	return names
}

func extractPackageName(line string) string {
	for _, separator := range []string{"===", "==", "!=", "<=", ">=", "~=", ">", "<"} {
		if i := strings.Index(line, separator); i > 0 {
			return strings.TrimSpace(line[:i])
		}
	}

	return strings.TrimSpace(line)
}

type pyproject struct {
	Project *struct {
		RequiresPython       string              `toml:"requires-python"`
		Dependencies         []string            `toml:"dependencies"`
		OptionalDependencies map[string][]string `toml:"optional-dependencies"`
	} `toml:"project"`
	Tool struct {
		Poetry struct {
			Dependencies    map[string]any `toml:"dependencies"`
			DevDependencies map[string]any `toml:"dev-dependencies"`
			Group           map[string]struct {
				Dependencies map[string]any `toml:"dependencies"`
			} `toml:"group"`
		} `toml:"poetry"`
	} `toml:"tool"`
}

func (p pyproject) requiresPython() string {
	if p.Project != nil && p.Project.RequiresPython != "" {
		return p.Project.RequiresPython
	}

	if constraint, ok := p.Tool.Poetry.Dependencies["python"].(string); ok {
		return constraint
	}

	return ""
}

func loadPyproject(rc ecosystem.RunContext) (pyproject, error) {
	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "pyproject.toml"))
	if err != nil {
		return pyproject{}, fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	var doc pyproject

	if err := toml.Unmarshal(raw, &doc); err != nil {
		return pyproject{}, nil
	}

	return doc, nil
}

func loadPoetryPyproject(rc ecosystem.RunContext) (main, dev, optional []string, requiresPython string, err error) {
	doc, err := loadPyproject(rc)
	if err != nil {
		return nil, nil, nil, "", err
	}

	main, optional = splitPoetryDeps(doc.Tool.Poetry.Dependencies)
	dev = poetryDevDeps(doc)

	return dedupeSorted(main), dedupeSorted(dev), dedupeSorted(optional), doc.requiresPython(), nil
}

func splitPoetryDeps(deps map[string]any) (required, optional []string) {
	for name, value := range deps {
		if strings.EqualFold(name, "python") {
			continue
		}

		if poetryDepOptional(value) {
			optional = append(optional, name)
		} else {
			required = append(required, name)
		}
	}

	return required, optional
}

func poetryDepOptional(value any) bool {
	table, ok := value.(map[string]any)
	if !ok {
		return false
	}

	optional, _ := table["optional"].(bool)

	return optional
}

func poetryDevDeps(doc pyproject) []string {
	if group, ok := doc.Tool.Poetry.Group["dev"]; ok && len(group.Dependencies) > 0 {
		return mapKeys(group.Dependencies)
	}

	return mapKeys(doc.Tool.Poetry.DevDependencies)
}

func mapKeys(deps map[string]any) []string {
	keys := make([]string, 0, len(deps))

	for name := range deps {
		keys = append(keys, name)
	}

	return keys
}

func loadPEP621Pyproject(rc ecosystem.RunContext) (main, dev, optional []string, requiresPython string, err error) {
	doc, err := loadPyproject(rc)
	if err != nil {
		return nil, nil, nil, "", err
	}

	if doc.Project == nil {
		return nil, nil, nil, doc.requiresPython(), errors.New("pyproject.toml has no [project] section")
	}

	main = pep508Names(doc.Project.Dependencies)

	for group, specs := range doc.Project.OptionalDependencies {
		names := pep508Names(specs)

		if _, isDev := pep621DevExtraNames[strings.ToLower(group)]; isDev {
			dev = append(dev, names...)
		} else {
			optional = append(optional, names...)
		}
	}

	return dedupeSorted(main), dedupeSorted(dev), dedupeSorted(optional), doc.requiresPython(), nil
}

func pep508Names(specs []string) []string {
	names := make([]string, 0, len(specs))

	for _, spec := range specs {
		match := pep508NameFromLine.FindStringSubmatch(strings.TrimSpace(spec))

		if len(match) < 2 {
			continue
		}

		names = append(names, match[1])
	}

	return names
}

func loadPipfileDeps(rc ecosystem.RunContext) (main, dev []string, err error) {
	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "Pipfile"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read Pipfile: %w", err)
	}

	var doc struct {
		Packages    map[string]any `toml:"packages"`
		DevPackages map[string]any `toml:"dev-packages"`
	}

	if err := toml.Unmarshal(raw, &doc); err != nil {
		return nil, nil, nil
	}

	return dedupeSorted(mapKeys(doc.Packages)), dedupeSorted(mapKeys(doc.DevPackages)), nil
}

func dedupeSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}

	seen := make(map[string]struct{})

	var output []string

	for _, item := range in {
		key := strings.ToLower(item)

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		output = append(output, item)
	}

	slices.Sort(output)

	return output
}

func pythonRuntimeVersion(ctx context.Context, rc ecosystem.RunContext) (string, error) {
	result, err := rc.Runner.Run(ctx, "python", "--version")
	if err != nil {
		result, err = rc.Runner.Run(ctx, "python3", "--version")
	}

	if err != nil {
		return "", err
	}

	matches := pythonSemverFromVersion.FindStringSubmatch(result.Stdout)

	if len(matches) < 2 {
		return "", fmt.Errorf("unexpected python version output: %s", result.Stdout)
	}

	return matches[1], nil
}

func runPipCheck(ctx context.Context, rc ecosystem.RunContext, command string) error {
	check := []string{"-m", "pip", "check"}

	switch command {
	case "poetry":
		_, err := rc.Runner.Run(ctx, "poetry", append([]string{"run", "python"}, check...)...)
		return err
	case "uv":
		_, err := rc.Runner.Run(ctx, "uv", "pip", "check")
		return err
	case "pipenv":
		_, err := rc.Runner.Run(ctx, "pipenv", append([]string{"run", "python"}, check...)...)
		return err
	case "pdm":
		_, err := rc.Runner.Run(ctx, "pdm", append([]string{"run", "python"}, check...)...)
		return err
	default:
		_, err := rc.Runner.Run(ctx, "python", check...)
		if err != nil {
			_, err2 := rc.Runner.Run(ctx, "python3", check...)
			return err2
		}

		return nil
	}
}

func installedPackages(ctx context.Context, rc ecosystem.RunContext, command string) map[string]string {
	switch command {
	case "poetry":
		result, err := rc.Runner.Run(ctx, "poetry", "show", "--format", "json")
		if err != nil {
			return map[string]string{}
		}

		return parsePipListJSON(result.Stdout)
	case "uv":
		result, err := rc.Runner.Run(ctx, "uv", "pip", "list", "--format=json")
		if err != nil {
			return map[string]string{}
		}

		return parsePipListJSON(result.Stdout)
	case "pipenv":
		return pipListMap(ctx, rc, []string{"pipenv", "run"})
	case "pdm":
		result, err := rc.Runner.Run(ctx, "pdm", "list", "--json")
		if err != nil {
			return map[string]string{}
		}

		return parsePipListJSON(result.Stdout)
	default:
		return pipListMap(ctx, rc, nil)
	}
}

func pipListMap(ctx context.Context, rc ecosystem.RunContext, prefix []string) map[string]string {
	output, err := runPipListJSON(ctx, rc, prefix)
	if err != nil {
		return map[string]string{}
	}

	return parsePipListJSON(output)
}

func runPipListJSON(ctx context.Context, rc ecosystem.RunContext, prefix []string) (string, error) {
	args := []string{"-m", "pip", "list", "--format=json"}

	if len(prefix) == 0 {
		result, err := rc.Runner.Run(ctx, "python", args...)
		if err != nil {
			fallback, fallbackErr := rc.Runner.Run(ctx, "python3", args...)
			return fallback.Stdout, fallbackErr
		}

		return result.Stdout, nil
	}

	fullArgs := append(append([]string{}, prefix...), append([]string{"python"}, args...)...)

	result, err := rc.Runner.Run(ctx, fullArgs[0], fullArgs[1:]...)

	return result.Stdout, err
}

func parsePipListJSON(output string) map[string]string {
	var entries []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	if json.Unmarshal([]byte(output), &entries) != nil {
		return map[string]string{}
	}

	packages := make(map[string]string, len(entries))

	for _, entry := range entries {
		if entry.Name == "" {
			continue
		}

		packages[strings.ToLower(entry.Name)] = entry.Version
	}

	return packages
}

func outdatedParser(command string, parse func(string) ([]ecosystem.OutdatedPackage, error)) ecosystem.OutdatedParser {
	return func(rc ecosystem.RunContext, stdout string) ([]ecosystem.OutdatedPackage, error) {
		packages, err := parse(stdout)
		if err != nil {
			return nil, err
		}

		config := loadConfig(rc, command)
		direct := ecosystem.ToSet(config.Dependencies, config.DevDependencies, config.OptionalDependencies)

		return ecosystem.FilterDirect(packages, direct), nil
	}
}

func parsePDMOutdated(output string) ([]ecosystem.OutdatedPackage, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	var entries []struct {
		Package          string `json:"package"`
		InstalledVersion string `json:"installed_version"`
		LatestVersion    string `json:"latest_version"`
	}

	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		return nil, err
	}

	packages := make([]ecosystem.OutdatedPackage, 0, len(entries))

	for _, entry := range entries {
		packages = append(packages, ecosystem.OutdatedPackage{
			Name:    entry.Package,
			Current: entry.InstalledVersion,
			Latest:  entry.LatestVersion,
		})
	}

	ecosystem.SortOutdated(packages)

	return packages, nil
}

func parsePipOutdated(output string) ([]ecosystem.OutdatedPackage, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	var entries []struct {
		Name          string `json:"name"`
		Version       string `json:"version"`
		LatestVersion string `json:"latest_version"`
	}

	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		return nil, err
	}

	packages := make([]ecosystem.OutdatedPackage, 0, len(entries))

	for _, entry := range entries {
		packages = append(packages, ecosystem.OutdatedPackage{
			Name:    entry.Name,
			Current: entry.Version,
			Latest:  entry.LatestVersion,
		})
	}

	ecosystem.SortOutdated(packages)

	return packages, nil
}

func parsePipAuditFindings(jsonText string) []model.Finding {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "[") {
		return nil
	}

	var packages []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Vulns   []struct {
			ID          string   `json:"id"`
			FixVersions []string `json:"fix_versions"`
			Aliases     []string `json:"aliases"`
			Description string   `json:"description"`
		} `json:"vulns"`
	}

	if err := json.Unmarshal([]byte(jsonText), &packages); err != nil {
		return nil
	}

	var findings []model.Finding

	for _, pkg := range packages {
		for _, vuln := range pkg.Vulns {
			fixedIn := ""
			if len(vuln.FixVersions) > 0 {
				fixedIn = vuln.FixVersions[0]
			}

			findings = append(findings, model.Finding{
				ID:       vuln.ID,
				Aliases:  vuln.Aliases,
				Severity: "high", // pip-audit does not report a severity
				Package:  pkg.Name,
				Version:  pkg.Version,
				FixedIn:  fixedIn,
				Summary:  ecosystem.FirstLine(vuln.Description),
			})
		}
	}

	ecosystem.SortFindings(findings)

	return findings
}

// parseUvAuditFindings reads uv audit's summary count. uv only exposes the total
// vulnerability count in a documented field today, so each finding carries just
// a severity until uv stabilizes a per-vulnerability JSON shape.
func parseUvAuditFindings(jsonText string) []model.Finding {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" {
		return nil
	}

	var report struct {
		Summary struct {
			Vulnerabilities int `json:"vulnerabilities"`
		} `json:"summary"`
	}

	if err := json.Unmarshal([]byte(jsonText), &report); err != nil {
		return nil
	}

	if report.Summary.Vulnerabilities <= 0 {
		return nil
	}

	findings := make([]model.Finding, report.Summary.Vulnerabilities)

	for i := range findings {
		findings[i] = model.Finding{Severity: "high"}
	}

	return findings
}
