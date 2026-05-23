package composer

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/parallel"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func Spec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:        "composer",
		DisplayName: "Composer",
		Priority:    2,
		Managers: []ecosystem.Manager{{
			Command:     "composer",
			DisplayName: "Composer",
			ConfigFile:  "composer.json",
			LockFile:    "composer.lock",
			VersionArgs: []string{"--version"},
			InstallArgs: []string{"install"},
			ForceArgs:   []string{"--no-cache"},
			Outdated: &ecosystem.OutdatedProbe{
				Args:  []string{"outdated", "--direct", "--format=json"},
				Parse: parseOutdated,
			},
			Audit: &ecosystem.AuditProbe{
				Args:  []string{"audit", "--format=json"},
				Parse: parseComposerAdvisoryCounts,
			},
		}},
		Check: check,
	}
}

type depVersion struct {
	name    string
	version string
}

func check(ctx context.Context, rc ecosystem.RunContext, detection ecosystem.Detection) []model.Message {
	if ctx.Err() != nil {
		return nil
	}

	if !rc.FileExists("composer.json") {
		return warnsMissingComposerJSON(detection.Active.LockFile)
	}

	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "composer.json"))
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Failed to read composer.json: %v", err)}}
	}

	dependencies, devDependencies, err := parseComposerJSON(raw)
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Failed to read composer.json: %v", err)}}
	}

	composerVersion, err := readComposerVersion(ctx, rc)
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Composer is not installed or not on PATH: %v", err)}}
	}

	messages := []model.Message{
		{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %sComposer (%s)", terminal.Reset, composerVersion)},
		{Severity: model.SeveritySuccess, Text: "composer.json found:"},
	}

	installed := installedDependencies(ctx, rc, dependencies, devDependencies)

	for _, dep := range slices.Concat(dependencies, devDependencies) {
		isDev := slices.Contains(devDependencies, dep)

		if version, ok := installed[dep]; ok {
			messages = append(messages, model.Message{
				Severity: model.SeveritySuccess,
				Text:     fmt.Sprintf("Installed dependency %s%s (%s)", terminal.Reset, dep, version),
				Nested:   true,
				Dev:      isDev,
			})
		} else {
			messages = append(messages, model.Message{
				Severity: model.SeverityError,
				Text:     fmt.Sprintf("Missing dependency %s%s, Run `composer require %s`", terminal.Reset, dep, dep),
				Nested:   true,
				Dev:      isDev,
			})
		}
	}

	return messages
}

func warnsMissingComposerJSON(lockFile string) []model.Message {
	warnings := []model.Message{{Severity: model.SeverityWarning, Text: "composer.json not found."}}

	if lockFile != "" {
		warnings = append(warnings, model.Message{
			Severity: model.SeverityWarning,
			Text:     fmt.Sprintf("composer.json not found, but %s exists. Ensure composer.json is included in your project.", lockFile),
		})
	}

	return warnings
}

func parseComposerJSON(raw []byte) (dependencies, devDependencies []string, err error) {
	var data struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}

	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, nil, err
	}

	for dep := range data.Require {
		if dep == "php" || strings.HasPrefix(dep, "ext-") {
			continue
		}

		dependencies = append(dependencies, dep)
	}

	for dep := range data.RequireDev {
		if strings.HasPrefix(dep, "ext-") {
			continue
		}

		devDependencies = append(devDependencies, dep)
	}

	slices.Sort(dependencies)
	slices.Sort(devDependencies)

	return dependencies, devDependencies, nil
}

func parseOutdated(_ ecosystem.RunContext, output string) ([]ecosystem.OutdatedPackage, error) {
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

	packages := make([]ecosystem.OutdatedPackage, 0, len(data.Installed))

	for _, pkg := range data.Installed {
		if pkg.Version == pkg.Latest {
			continue
		}

		packages = append(packages, ecosystem.OutdatedPackage{
			Name:    pkg.Name,
			Current: pkg.Version,
			Latest:  pkg.Latest,
		})
	}

	slices.SortFunc(packages, func(a, b ecosystem.OutdatedPackage) int {
		return strings.Compare(a.Name, b.Name)
	})

	return packages, nil
}

func readComposerVersion(ctx context.Context, rc ecosystem.RunContext) (string, error) {
	result, err := rc.Runner.Run(ctx, "composer", "--version")
	if err != nil {
		return "", fmt.Errorf("failed to run composer --version: %w", err)
	}

	parts := strings.Fields(result.Stdout)

	if len(parts) >= 3 {
		return parts[2], nil
	}

	return "", fmt.Errorf("unexpected composer version format: %s", result.Stdout)
}

func installedDependencies(ctx context.Context, rc ecosystem.RunContext, dependencies, devDependencies []string) map[string]string {
	required := slices.Concat(dependencies, devDependencies)
	installed := installedFromJSON(ctx, rc)

	if len(required) == 0 {
		return installed
	}

	fillMissingDeps(ctx, rc, installed, required)

	return installed
}

func installedFromJSON(ctx context.Context, rc ecosystem.RunContext) map[string]string {
	installed := make(map[string]string)

	result, err := rc.Runner.Run(ctx, "composer", "show", "--format=json")
	if err != nil {
		return installed
	}

	var data struct {
		Dependencies []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"installed"`
	}

	if json.Unmarshal([]byte(result.Stdout), &data) != nil {
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

func fillMissingDeps(ctx context.Context, rc ecosystem.RunContext, installed map[string]string, required []string) {
	var missing []string

	for _, dep := range required {
		if dep == "" {
			continue
		}

		if _, exists := installed[dep]; exists {
			continue
		}

		missing = append(missing, dep)
	}

	found := parallel.Collect(ctx, missing, func(ctx context.Context, dep string) (depVersion, bool) {
		version := depVersionFromShow(ctx, rc, dep)

		if version == "" {
			return depVersion{}, false
		}

		return depVersion{name: dep, version: version}, true
	})

	for _, pkg := range found {
		installed[pkg.name] = pkg.version
	}
}

func depVersionFromShow(ctx context.Context, rc ecosystem.RunContext, dep string) string {
	result, err := rc.Runner.Run(ctx, "composer", "show", dep)
	if err != nil {
		return ""
	}

	for line := range strings.SplitSeq(result.Stdout, "\n") {
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

func parseComposerAdvisoryCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "{") {
		return nil
	}

	var root struct {
		Advisories map[string][]struct {
			Severity string `json:"severity"`
		} `json:"advisories"`
	}

	if err := json.Unmarshal([]byte(jsonText), &root); err != nil {
		return nil
	}

	counts := make(map[string]int)

	for _, advisories := range root.Advisories {
		for _, advisory := range advisories {
			severity := strings.ToLower(strings.TrimSpace(advisory.Severity))

			if severity == "" {
				severity = "medium"
			}

			counts[severity]++
		}
	}

	return counts
}
