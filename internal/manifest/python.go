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
	RequiresPython           string // effective constraint (pyproject or .python-version fallback)
	RequiresPythonConstraint string // from pyproject.toml only
	PythonVersionPin         string // raw .python-version content
	HasConfig                bool
	Error                    error
}

var (
	pep508NameFromLine = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?)`)
	requiresPythonRe   = regexp.MustCompile(`(?m)^requires-python\s*=\s*["']([^"']+)["']`)
	pythonVersionRe    = regexp.MustCompile(`(?m)^python\s*=\s*["']([^"']+)["']`)
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
		config.Dependencies, config.DevDependencies, config.RequiresPythonConstraint, err = l.loadPoetryPyproject()
	case "uv", "pdm":
		config.Dependencies, config.DevDependencies, config.RequiresPythonConstraint, err = l.loadPEP621Pyproject()
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
		if index := strings.Index(line, separator); index > 0 {
			return strings.TrimSpace(line[:index])
		}
	}

	return strings.TrimSpace(line)
}

func (l Loader) loadPoetryPyproject() (main, dev []string, requiresPython string, err error) {
	raw, err := l.readFile("pyproject.toml")

	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to read pyproject.toml: %w", err)
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

	main = parseTomlKeyValueSection(content, "[tool.poetry.dependencies]", true)
	dev = parseTomlKeyValueSection(content, "[tool.poetry.group.dev.dependencies]", false)

	if len(dev) == 0 {
		dev = parseTomlKeyValueSection(content, "[tool.poetry.dev-dependencies]", false)
	}

	return dedupeSorted(main), dedupeSorted(dev), requiresPython, nil
}

func (l Loader) loadPEP621Pyproject() (main, dev []string, requiresPython string, err error) {
	raw, err := l.readFile("pyproject.toml")

	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	content := string(raw)
	requiresPython = extractRequiresPythonConstraint(content)

	project := extractTomlSection(content, "[project]")

	if project == "" {
		return nil, nil, requiresPython, errors.New("pyproject.toml has no [project] section")
	}

	main = parseTomlArray(project, `(?m)^dependencies\s*=\s*\[`)

	if opt := extractTomlSection(content, "[project.optional-dependencies]"); opt != "" {
		for _, group := range []string{"dev", "test", "docs"} {
			dev = append(dev, parseTomlArray(opt, fmt.Sprintf(`(?m)^%s\s*=\s*\[`, regexp.QuoteMeta(group)))...)
		}
	}

	return dedupeSorted(main), dedupeSorted(dev), requiresPython, nil
}

func (l Loader) loadPipfileDeps() (main, dev []string, err error) {
	raw, err := l.readFile("Pipfile")

	if err != nil {
		return nil, nil, fmt.Errorf("failed to read Pipfile: %w", err)
	}

	content := string(raw)
	main = parseTomlKeyValueSection(content, "[packages]", false)
	dev = parseTomlKeyValueSection(content, "[dev-packages]", false)

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

// parseTomlKeyValueSection parses sections like `[packages]`. skipPython excludes "python" entries.
func parseTomlKeyValueSection(full, header string, skipPython bool) []string {
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

		if skipPython && strings.EqualFold(name, "python") {
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
	index := strings.Index(s, header)

	if index < 0 {
		return ""
	}

	section := s[index:]

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
