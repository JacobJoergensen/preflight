package manifest

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type PythonConfig struct {
	PackageManager           PackageManager
	Dependencies             []string
	DevDependencies          []string
	OptionalDependencies     []string
	RequiresPython           string // effective constraint (pyproject or .python-version fallback)
	RequiresPythonConstraint string // from pyproject.toml only
	PythonVersionPin         string // raw .python-version content
	HasConfig                bool
	Error                    error
}

var (
	pep508NameFromLine   = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?)`)
	requiresPythonRe     = regexp.MustCompile(`(?m)^requires-python\s*=\s*["']([^"']+)["']`)
	pythonVersionRe      = regexp.MustCompile(`(?m)^python\s*=\s*["']([^"']+)["']`)
	pep621ExtraNameRe    = regexp.MustCompile(`(?m)^([a-zA-Z][a-zA-Z0-9_-]*)\s*=\s*\[`)
	pep621DevExtraNames  = map[string]struct{}{"dev": {}, "test": {}, "docs": {}}
	poetryOptionalMarker = regexp.MustCompile(`(?:^|[,{ \t])optional\s*=\s*true(?:[\s,}]|$)`)
)

func (l Loader) LoadPythonConfig() PythonConfig {
	config := PythonConfig{}
	config.PackageManager, _ = l.DetectPackageManager(PackageTypePython)
	config.HasConfig = config.PackageManager.ConfigFileExists

	if !config.HasConfig {
		return config
	}

	var err error

	switch config.PackageManager.Command() {
	case "pip":
		config.Dependencies, err = l.loadRequirementsTxtDeps("requirements.txt")
	case "poetry":
		config.Dependencies, config.DevDependencies, config.OptionalDependencies, config.RequiresPythonConstraint, err = l.loadPoetryPyproject()
	case "uv", "pdm":
		config.Dependencies, config.DevDependencies, config.OptionalDependencies, config.RequiresPythonConstraint, err = l.loadPEP621Pyproject()
	case "pipenv":
		config.Dependencies, config.DevDependencies, err = l.loadPipfileDeps()
	default:
		err = fmt.Errorf("unsupported python tool: %s", config.PackageManager.Command())
	}

	if err != nil {
		config.Error = err
	}

	config.PythonVersionPin = l.ReadPythonVersionPin()
	config.RequiresPython = config.RequiresPythonConstraint

	if config.RequiresPython == "" && config.PythonVersionPin != "" {
		config.RequiresPython = config.PythonVersionPin
	}

	return config
}

func (l Loader) loadRequirementsTxtDeps(filename string) ([]string, error) {
	raw, err := l.readFile(filename)

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

		// Strip extras like `package[extra]`
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

func (l Loader) loadPoetryPyproject() (main, dev, optional []string, requiresPython string, err error) {
	raw, err := l.readFile("pyproject.toml")

	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	content := string(raw)

	// Try [project] section first, then Poetry-specific
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

	depsSection := extractTomlSection(content, "[tool.poetry.dependencies]")
	main, optional = parsePoetryDependenciesSection(depsSection)

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

func (l Loader) loadPEP621Pyproject() (main, dev, optional []string, requiresPython string, err error) {
	raw, err := l.readFile("pyproject.toml")

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

func (l Loader) loadPipfileDeps() (main, dev []string, err error) {
	raw, err := l.readFile("Pipfile")

	if err != nil {
		return nil, nil, fmt.Errorf("failed to read Pipfile: %w", err)
	}

	content := string(raw)
	main = parseTomlKeyValueSection(content, "[packages]")
	dev = parseTomlKeyValueSection(content, "[dev-packages]")

	return dedupeSorted(main), dedupeSorted(dev), nil
}

// parseTomlArray parses arrays like `dependencies = [...]`.
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

// parseTomlKeyValueSection parses sections like `[packages]`.
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

		name := strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
		name = strings.Trim(name, `"'`)

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

func extractRequiresPythonConstraint(s string) string {
	if match := requiresPythonRe.FindStringSubmatch(s); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	if match := pythonVersionRe.FindStringSubmatch(s); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	return ""
}

func extractTomlSection(s, header string) string {
	i := strings.Index(s, header)

	if i < 0 {
		return ""
	}

	section := s[i:]

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
