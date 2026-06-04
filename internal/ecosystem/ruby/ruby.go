package ruby

import (
	"context"
	"encoding/csv"
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

var (
	gemfileGemLine  = regexp.MustCompile(`(?m)^\s*gem\s+['"]([^'"]+)['"]`)
	gemfileRubyLine = regexp.MustCompile(`(?m)^\s*ruby\s+['"]([^'"]+)['"]`)
	rubyVersionLine = regexp.MustCompile(`^(\d+\.\d+\.\d+)`)
	bundleOutdated  = regexp.MustCompile(`^\s*\*\s+([a-zA-Z0-9_.-]+)\s+\(newest\s+([^,]+),\s+installed\s+([^,)]+)`)
	bundleListLine  = regexp.MustCompile(`^\s*\*\s+([a-zA-Z0-9_.-]+)\s+\(([^)]+)\)`)
)

var managers = []ecosystem.Manager{{
	Command: "bundle", DisplayName: "Bundler", ConfigFile: "Gemfile", LockFile: "Gemfile.lock",
	VersionArgs: []string{"version"}, InstallArgs: []string{"install"}, ForceArgs: []string{"--redownload"},
	Outdated: &ecosystem.OutdatedProbe{Args: []string{"outdated"}, Parse: parseOutdated},
	Audit: &ecosystem.AuditProbe{
		Tool:            "bundle-audit",
		Args:            []string{"check"},
		ToolMissingHint: "bundle-audit not found on PATH (gem install bundler-audit)",
		Parse:           parseBundleAuditFindings,
	},
}}

var detectMarkers = []ecosystem.Marker{
	{File: "Gemfile", Manager: "bundle"},
	{File: "gems.rb", Manager: "bundle"},
}

func Spec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:            "ruby",
		DisplayName:     "Ruby",
		Priority:        8,
		Managers:        managers,
		RuntimeCommands: []string{"ruby"},
		Detect:          detectMarkers,
		Check:           check,
		License:         scanLicenses,
		VersionPins:     []string{".ruby-version"},
		EnvSignals:      []string{"RBENV_VERSION", "GEM_HOME", "RUBY_ROOT"},
	}
}

func scanLicenses(ctx context.Context, rc ecosystem.RunContext, _ ecosystem.Detection) ecosystem.LicenseResult {
	return ecosystem.RunLicenseCommand(
		ctx,
		rc,
		"license_finder",
		"license_finder not found on PATH (install: gem install license_finder)",
		parseLicenseFinderCSV,
		"report",
		"--format",
		"csv",
		"--columns",
		"name",
		"version",
		"licenses",
	)
}

func parseLicenseFinderCSV(output string) []ecosystem.PackageLicense {
	reader := csv.NewReader(strings.NewReader(output))
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil
	}

	var packages []ecosystem.PackageLicense

	for _, record := range records {
		if len(record) < 3 {
			continue
		}

		name := strings.TrimSpace(record[0])

		if name == "" || strings.EqualFold(name, "name") {
			continue
		}

		packages = append(packages, ecosystem.PackageLicense{
			Name:    name,
			Version: strings.TrimSpace(record[1]),
			License: strings.TrimSpace(record[2]),
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
	config := loadConfig(rc)

	if config.Error != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Failed to read Ruby project files: %v", config.Error)}}
	}

	var messages []model.Message

	if config.RubyVersionPin != "" && config.RequiresRubyFromGemfile != "" {
		pinCore := semver.ParseVersionPin(config.RubyVersionPin)

		if pinCore != "" && !semver.MatchVersionConstraint(pinCore, config.RequiresRubyFromGemfile) {
			messages = append(messages, model.Message{Severity: model.SeverityWarning, Text: fmt.Sprintf(
				".ruby-version pins %s but Gemfile specifies %s. Align these for consistent installs.",
				strings.TrimSpace(config.RubyVersionPin), strings.TrimSpace(config.RequiresRubyFromGemfile),
			)})
		}
	}

	if config.RequiresRuby != "" {
		rubyVersion, err := rubyRuntimeVersion(ctx, rc)
		if err != nil {
			messages = append(messages, model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("Ruby is not installed or not on PATH: %v", err)})
			return messages
		}

		if ok, msg := semver.ValidateVersion(rubyVersion, config.RequiresRuby); !ok {
			if msg != "" {
				messages = append(messages, model.Message{Severity: model.SeverityError, Text: msg})
			} else {
				messages = append(messages, model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("Ruby version %s does not satisfy %s", rubyVersion, config.RequiresRuby)})
			}
		}
	}

	installedVersion, err := ecosystem.ToolVersion(ctx, rc, manager)
	if err != nil {
		messages = append(messages, model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("%s is not installed or not on PATH: %v", manager.DisplayName, err)})
		return messages
	}

	rhs := "Gemfile.lock"

	if manager.LockFile != "" && rc.FileExists(manager.LockFile) {
		rhs = manager.LockFile
	}

	messages = append(
		messages,
		model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %s%s (%s ⟶ %s)", terminal.Reset, manager.DisplayName, ecosystem.FirstLine(installedVersion), rhs)},
	)

	if manager.LockFile != "" && rc.FileExists(manager.LockFile) {
		if _, err := rc.Runner.Run(ctx, "bundle", "check"); err != nil {
			messages = append(messages, model.Message{Severity: model.SeverityError, Text: ecosystem.FormatExecFailure("bundle check failed", err)})
		}
	}

	messages = append(messages, ecosystem.MissingLockfileWarning(rc, manager, len(config.Dependencies) > 0)...)

	installed := installedGems(ctx, rc)

	seen := make(map[string]struct{})

	for _, dep := range config.Dependencies {
		if dep == "" {
			continue
		}

		key := strings.ToLower(dep)

		if _, dup := seen[key]; dup {
			continue
		}

		seen[key] = struct{}{}

		if gemVersion, ok := installed[key]; ok {
			messages = append(messages, model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed gem %s%s (%s)", terminal.Reset, dep, gemVersion), Nested: true})
		} else {
			messages = append(messages, model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("Missing gem %s%s, run `%s install`", terminal.Reset, dep, manager.Command), Nested: true})
		}
	}

	return messages
}

type rubyConfig struct {
	Dependencies            []string
	RequiresRuby            string // effective constraint (Gemfile or .ruby-version fallback)
	RequiresRubyFromGemfile string
	RubyVersionPin          string
	Error                   error
}

func loadConfig(rc ecosystem.RunContext) rubyConfig {
	gemfileName := "Gemfile"

	if !rc.FileExists("Gemfile") && rc.FileExists("gems.rb") {
		gemfileName = "gems.rb"
	}

	var config rubyConfig

	deps, requiresRuby, err := loadRubyDependencies(rc, gemfileName)
	if err != nil {
		config.Error = err
	}

	config.Dependencies = deps
	config.RequiresRubyFromGemfile = requiresRuby
	config.RubyVersionPin = ecosystem.ReadVersionPin(rc, ".ruby-version")
	config.RequiresRuby = config.RequiresRubyFromGemfile

	if config.RequiresRuby == "" && config.RubyVersionPin != "" {
		config.RequiresRuby = config.RubyVersionPin
	}

	return config
}

func loadRubyDependencies(rc ecosystem.RunContext, gemfileName string) (deps []string, requiresRuby string, err error) {
	rawGemfile, errGemfile := rc.FS.ReadFile(filepath.Join(rc.WorkDir, gemfileName))

	if errGemfile != nil {
		return nil, "", fmt.Errorf("failed to read %s: %w", gemfileName, errGemfile)
	}

	gemfileText := string(rawGemfile)
	requiresRuby = parseGemfileRubyVersion(gemfileText)

	if rawLock, lockErr := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "Gemfile.lock")); lockErr == nil {
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

	return slices.Compact(deps), requiresRuby, nil
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

func parseGemfileLockRubyVersion(lock string) string {
	lines := strings.Split(lock, "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) != "RUBY VERSION" {
			continue
		}

		if i+1 >= len(lines) {
			return ""
		}

		next := strings.TrimSpace(lines[i+1])

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
		if i := strings.Index(line, separator); i > 0 {
			return strings.TrimSpace(line[:i])
		}
	}

	return line
}

func rubyRuntimeVersion(ctx context.Context, rc ecosystem.RunContext) (string, error) {
	result, err := rc.Runner.Run(ctx, "ruby", "-e", "print RUBY_VERSION")
	if err != nil {
		return "", err
	}

	return result.Stdout, nil
}

func installedGems(ctx context.Context, rc ecosystem.RunContext) map[string]string {
	result, err := rc.Runner.Run(ctx, "bundle", "list")
	if err != nil {
		return map[string]string{}
	}

	gems := make(map[string]string)

	for line := range strings.SplitSeq(result.Stdout, "\n") {
		parts := bundleListLine.FindStringSubmatch(line)

		if len(parts) < 3 {
			continue
		}

		gems[strings.ToLower(strings.TrimSpace(parts[1]))] = strings.TrimSpace(parts[2])
	}

	return gems
}

func parseOutdated(rc ecosystem.RunContext, output string) ([]ecosystem.OutdatedPackage, error) {
	var packages []ecosystem.OutdatedPackage

	for line := range strings.SplitSeq(output, "\n") {
		parts := bundleOutdated.FindStringSubmatch(line)

		if len(parts) < 4 {
			continue
		}

		packages = append(packages, ecosystem.OutdatedPackage{
			Name:    strings.TrimSpace(parts[1]),
			Current: strings.TrimSpace(parts[3]),
			Latest:  strings.TrimSpace(parts[2]),
		})
	}

	ecosystem.SortOutdated(packages)

	config := loadConfig(rc)

	return ecosystem.FilterDirect(packages, ecosystem.ToSet(config.Dependencies)), nil
}

func parseBundleAuditFindings(output string) []model.Finding {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	var findings []model.Finding

	var current model.Finding

	open := false

	flush := func() {
		if open {
			if current.Severity == "" {
				current.Severity = "high"
			}

			findings = append(findings, current)
		}

		current = model.Finding{}
		open = false
	}

	for line := range strings.SplitSeq(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}

		value = strings.TrimSpace(value)

		switch strings.TrimSpace(key) {
		case "Name":
			flush()
			current.Package = value
			open = true
		case "Version":
			current.Version = value
		case "Advisory", "CVE", "GHSA":
			if current.ID == "" {
				current.ID = value
			} else {
				current.Aliases = append(current.Aliases, value)
			}
		case "Criticality":
			current.Severity = bundleCriticality(value)
		case "URL":
			current.URL = value
		case "Title":
			current.Summary = value
		case "Solution":
			current.FixedIn = strings.TrimSpace(strings.TrimPrefix(value, "upgrade to "))
		}
	}

	flush()

	ecosystem.SortFindings(findings)

	return findings
}

func bundleCriticality(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "moderate"
	case "low":
		return "low"
	default:
		return "high"
	}
}
