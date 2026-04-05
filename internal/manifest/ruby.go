package manifest

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type RubyConfig struct {
	PackageManager          PackageManager
	Dependencies            []string
	RequiresRuby            string // effective constraint (Gemfile or .ruby-version fallback)
	RequiresRubyFromGemfile string // from Gemfile/Gemfile.lock only
	RubyVersionPin          string // raw .ruby-version content
	HasConfig               bool
	HasLock                 bool
	Error                   error
}

var (
	gemfileGemLine  = regexp.MustCompile(`(?m)^\s*gem\s+['"]([^'"]+)['"]`)
	gemfileRubyLine = regexp.MustCompile(`(?m)^\s*ruby\s+['"]([^'"]+)['"]`)
	rubyVersionLine = regexp.MustCompile(`^(\d+\.\d+\.\d+)`)
)

func (l Loader) LoadRubyConfig() RubyConfig {
	config := RubyConfig{}
	config.PackageManager, _ = l.DetectPackageManager("ruby")
	config.HasConfig = config.PackageManager.ConfigFileExists
	config.HasLock = config.PackageManager.LockFileExists

	if !config.HasConfig {
		return config
	}

	gemfileName := "Gemfile"

	if !l.FileExists("Gemfile") && l.FileExists("gems.rb") {
		gemfileName = "gems.rb"
	}

	var err error

	config.Dependencies, config.RequiresRubyFromGemfile, err = l.loadRubyDependencies(gemfileName)

	if err != nil {
		config.Error = err
	}

	config.RubyVersionPin = l.ReadRubyVersionPin()
	config.RequiresRuby = config.RequiresRubyFromGemfile

	if config.RequiresRuby == "" && config.RubyVersionPin != "" {
		config.RequiresRuby = config.RubyVersionPin
	}

	return config
}

func (l Loader) loadRubyDependencies(gemfileName string) (deps []string, requiresRuby string, err error) {
	rawGemfile, errGemfile := l.readFile(gemfileName)

	if errGemfile != nil {
		return nil, "", fmt.Errorf("failed to read %s: %w", gemfileName, errGemfile)
	}

	gemfileText := string(rawGemfile)
	requiresRuby = parseGemfileRubyVersion(gemfileText)

	if rawLock, err := l.readFile("Gemfile.lock"); err == nil {
		lockText := string(rawLock)
		deps = parseGemfileLockDependencies(lockText)

		if requiresRuby == "" {
			requiresRuby = parseGemfileLockRubyVersion(lockText)
		}
	}

	if len(deps) == 0 {
		deps = parseGemfileGemNames(gemfileText)
	}

	slices.Sort(deps)
	deps = slices.Compact(deps)

	return deps, requiresRuby, nil
}

func parseGemfileLockRubyVersion(lock string) string {
	lines := strings.Split(lock, "\n")

	for index, line := range lines {
		if strings.TrimSpace(line) != "RUBY VERSION" {
			continue
		}

		if index+1 >= len(lines) {
			return ""
		}

		next := strings.TrimSpace(lines[index+1])

		if !strings.HasPrefix(next, "ruby ") {
			return ""
		}

		version := strings.TrimSpace(strings.TrimPrefix(next, "ruby "))

		if match := rubyVersionLine.FindStringSubmatch(version); len(match) > 1 {
			return match[1]
		}
	}

	return ""
}

func parseGemfileRubyVersion(gemfile string) string {
	if match := gemfileRubyLine.FindStringSubmatch(gemfile); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	return ""
}

func parseGemfileGemNames(gemfile string) []string {
	matches := gemfileGemLine.FindAllStringSubmatch(gemfile, -1)
	seen := make(map[string]struct{})
	var names []string

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		name := strings.TrimSpace(match[1])

		if name == "" {
			continue
		}

		lower := strings.ToLower(name)

		if _, duplicate := seen[lower]; duplicate {
			continue
		}

		seen[lower] = struct{}{}
		names = append(names, name)
	}

	return names
}

func parseGemfileLockDependencies(lock string) []string {
	lines := strings.Split(lock, "\n")
	inDeps := false
	seen := make(map[string]struct{})
	var names []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "DEPENDENCIES" {
			inDeps = true

			continue
		}

		if !inDeps {
			continue
		}

		if trimmed == "" {
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}

		name := gemNameFromDependencyLine(trimmed)

		if name == "" {
			continue
		}

		lower := strings.ToLower(name)

		if _, duplicate := seen[lower]; duplicate {
			continue
		}

		seen[lower] = struct{}{}
		names = append(names, name)
	}

	return names
}

func gemNameFromDependencyLine(line string) string {
	line = strings.TrimSpace(line)

	if line == "" {
		return ""
	}

	for _, separator := range []string{" ", "(", ","} {
		if index := strings.Index(line, separator); index > 0 {
			return strings.TrimSpace(line[:index])
		}
	}

	return line
}
