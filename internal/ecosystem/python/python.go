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

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

var pipAudit = &ecosystem.AuditProbe{
	Tool:            "pip-audit",
	Args:            []string{"--format", "json"},
	ToolMissingHint: "pip-audit not found on PATH (install: pip install pip-audit)",
	Parse:           parsePipAuditCounts,
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
		Audit:    &ecosystem.AuditProbe{Args: []string{"audit", "--format", "json"}, Parse: parseUvAuditCounts},
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
	requiresPythonRe        = regexp.MustCompile(`(?m)^requires-python\s*=\s*["']([^"']+)["']`)
	pythonVersionRe         = regexp.MustCompile(`(?m)^python\s*=\s*["']([^"']+)["']`)
	pep621ExtraNameRe       = regexp.MustCompile(`(?m)^([a-zA-Z][a-zA-Z0-9_-]*)\s*=\s*\[`)
	pep621DevExtraNames     = map[string]struct{}{"dev": {}, "test": {}, "docs": {}}
	poetryOptionalMarker    = regexp.MustCompile(`(?:^|[,{ \t])optional\s*=\s*true(?:[\s,}]|$)`)
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
		VersionPins:     []string{".python-version"},
		EnvSignals:      []string{"VIRTUAL_ENV", "CONDA_PREFIX"},
	}
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

func loadPoetryPyproject(rc ecosystem.RunContext) (main, dev, optional []string, requiresPython string, err error) {
	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "pyproject.toml"))
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	content := string(raw)

	if project := extractTomlSection(content, "[project]"); project != "" {
		if match := requiresPythonRe.FindStringSubmatch(project); len(match) > 1 {
			requiresPython = strings.TrimSpace(match[1])
		}
	}

	if requiresPython == "" {
		if dep := extractTomlSection(content, "[tool.poetry.dependencies]"); dep != "" {
			if match := pythonVersionRe.FindStringSubmatch(dep); len(match) > 1 {
				requiresPython = strings.TrimSpace(match[1])
			}
		}
	}

	main, optional = parsePoetryDependenciesSection(extractTomlSection(content, "[tool.poetry.dependencies]"))

	dev = parseTomlKeyValueSection(content, "[tool.poetry.group.dev.dependencies]")

	if len(dev) == 0 {
		dev = parseTomlKeyValueSection(content, "[tool.poetry.dev-dependencies]")
	}

	return dedupeSorted(main), dedupeSorted(dev), dedupeSorted(optional), requiresPython, nil
}

func parsePoetryDependenciesSection(section string) (required, optional []string) {
	if section == "" {
		return nil, nil
	}

	if newline := strings.Index(section, "\n"); newline >= 0 {
		section = section[newline+1:]
	}

	for line := range strings.SplitSeq(section, "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])

		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}

		eq := strings.Index(line, "=")

		if eq <= 0 {
			continue
		}

		name := strings.Trim(strings.TrimSpace(line[:eq]), `"'`)

		if name == "" || strings.EqualFold(name, "python") {
			continue
		}

		value := strings.TrimSpace(line[eq+1:])

		if strings.HasPrefix(value, "{") && poetryOptionalMarker.MatchString(value) {
			optional = append(optional, name)
		} else {
			required = append(required, name)
		}
	}

	return required, optional
}

func loadPEP621Pyproject(rc ecosystem.RunContext) (main, dev, optional []string, requiresPython string, err error) {
	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "pyproject.toml"))
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	content := string(raw)
	requiresPython = extractRequiresPythonConstraint(content)

	project := extractTomlSection(content, "[project]")

	if project == "" {
		return nil, nil, nil, requiresPython, errors.New("pyproject.toml has no [project] section")
	}

	main = parseTomlArray(project, `(?m)^dependencies\s*=\s*\[`)

	if extras := extractTomlSection(content, "[project.optional-dependencies]"); extras != "" {
		for _, extraName := range extraGroupNames(extras) {
			pattern := fmt.Sprintf(`(?m)^%s\s*=\s*\[`, regexp.QuoteMeta(extraName))
			items := parseTomlArray(extras, pattern)

			if _, isDev := pep621DevExtraNames[strings.ToLower(extraName)]; isDev {
				dev = append(dev, items...)
			} else {
				optional = append(optional, items...)
			}
		}
	}

	return dedupeSorted(main), dedupeSorted(dev), dedupeSorted(optional), requiresPython, nil
}

func extraGroupNames(section string) []string {
	matches := pep621ExtraNameRe.FindAllStringSubmatch(section, -1)
	names := make([]string, 0, len(matches))

	for _, match := range matches {
		if len(match) > 1 {
			names = append(names, match[1])
		}
	}

	return names
}

func loadPipfileDeps(rc ecosystem.RunContext) (main, dev []string, err error) {
	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "Pipfile"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read Pipfile: %w", err)
	}

	content := string(raw)
	main = parseTomlKeyValueSection(content, "[packages]")
	dev = parseTomlKeyValueSection(content, "[dev-packages]")

	return dedupeSorted(main), dedupeSorted(dev), nil
}

func parseTomlArray(content, anchorPattern string) []string {
	regex := regexp.MustCompile(anchorPattern)
	location := regex.FindStringIndex(content)

	if location == nil {
		return nil
	}

	substring := content[location[1]-1:]
	open := strings.Index(substring, "[")

	if open < 0 {
		return nil
	}

	closeIdx := strings.Index(substring[open:], "]")

	if closeIdx < 0 {
		return nil
	}

	inner := substring[open+1 : open+closeIdx]

	var output []string

	seen := make(map[string]struct{})

	for part := range strings.SplitSeq(inner, ",") {
		part = strings.Trim(strings.TrimSpace(part), `"'`)

		if part == "" {
			continue
		}

		match := pep508NameFromLine.FindStringSubmatch(part)

		if len(match) < 2 {
			continue
		}

		lower := strings.ToLower(match[1])

		if _, ok := seen[lower]; ok {
			continue
		}

		seen[lower] = struct{}{}
		output = append(output, match[1])
	}

	slices.Sort(output)

	return output
}

func parseTomlKeyValueSection(full, header string) []string {
	_, section, ok := strings.Cut(full, header)

	if !ok {
		return nil
	}

	if end := strings.Index(section, "\n["); end >= 0 {
		section = section[:end]
	}

	var names []string

	seen := make(map[string]struct{})

	for line := range strings.SplitSeq(section, "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])

		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}

		name := strings.Trim(strings.TrimSpace(strings.SplitN(line, "=", 2)[0]), `"'`)

		if name == "" {
			continue
		}

		lower := strings.ToLower(name)

		if _, ok := seen[lower]; ok {
			continue
		}

		seen[lower] = struct{}{}
		names = append(names, name)
	}

	return names
}

func extractRequiresPythonConstraint(content string) string {
	if match := requiresPythonRe.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	if match := pythonVersionRe.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	return ""
}

func extractTomlSection(content, header string) string {
	i := strings.Index(content, header)

	if i < 0 {
		return ""
	}

	section := content[i:]

	if end := strings.Index(section[1:], "\n["); end >= 0 {
		return section[:end+1]
	}

	return section
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

	slices.SortFunc(packages, func(a, b ecosystem.OutdatedPackage) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

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

	slices.SortFunc(packages, func(a, b ecosystem.OutdatedPackage) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return packages, nil
}

func parsePipAuditCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "[") {
		return nil
	}

	var packages []struct {
		Vulns []struct {
			ID string `json:"id"`
		} `json:"vulns"`
	}

	if err := json.Unmarshal([]byte(jsonText), &packages); err != nil {
		return nil
	}

	vulnCount := 0

	for _, pkg := range packages {
		vulnCount += len(pkg.Vulns)
	}

	if vulnCount == 0 {
		return nil
	}

	return map[string]int{"high": vulnCount}
}

func parseUvAuditCounts(jsonText string) map[string]int {
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

	if report.Summary.Vulnerabilities == 0 {
		return nil
	}

	return map[string]int{"high": report.Summary.Vulnerabilities}
}
