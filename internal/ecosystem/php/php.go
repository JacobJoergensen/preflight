package php

import (
	"cmp"
	"context"
	"encoding/json"
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
	phpVersionRegex = regexp.MustCompile(`PHP (\d+\.\d+\.\d+)`)
	moduleNameRegex = regexp.MustCompile(`^[A-Za-z0-9_ ]+$`)

	deprecatedExtensions = map[string]struct{}{
		"imap": {}, "mysql": {}, "recode": {}, "statistics": {}, "wddx": {}, "xml-rpc": {},
	}

	experimentalExtensions = map[string]struct{}{
		"gmagick": {}, "imagemagick": {}, "mqseries": {}, "parle": {}, "rnp": {},
		"svm": {}, "svn": {}, "ui": {}, "omq": {},
	}
)

type extensionInfo struct {
	Name      string
	Source    string
	IsWarning bool
	Warning   string
}

func Spec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:            "php",
		DisplayName:     "PHP",
		Priority:        1,
		RuntimeCommands: []string{"php", "pie"},
		Detect:          []ecosystem.Marker{{File: "composer.json"}, {File: "composer.lock"}},
		Check:           check,
	}
}

func check(ctx context.Context, rc ecosystem.RunContext, _ ecosystem.Detection) []model.Message {
	if ctx.Err() != nil {
		return nil
	}

	requiredVersion, requiredExtensions, err := phpContextFromComposer(rc)
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Failed to read composer.json: %v", err)}}
	}

	phpVersion, err := phpRuntimeVersion(ctx, rc)
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("PHP is not installed or not on PATH: %v", err)}}
	}

	var messages []model.Message

	if requiredVersion != "" {
		satisfied, _ := semver.ValidateVersion(phpVersion, requiredVersion)
		messages = append(messages, ecosystem.VersionFeedback("php", "PHP", phpVersion, requiredVersion, semver.MajorMinor(phpVersion), satisfied)...)
	}

	installedExtensions, err := phpExtensions(ctx, rc)
	if err != nil {
		messages = append(messages, model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("Failed to check PHP extensions: %v", err)})
		return messages
	}

	extensionSources := make(map[string]string)

	for ext := range installedExtensions {
		extensionSources[ext] = "php"
	}

	pie := loadPieConfig(ctx, rc)
	pieExtensions := make(map[string]struct{})

	if semver.MatchVersionConstraint(phpVersion, ">=8.4") && pie.IsInstalled {
		for _, ext := range pie.Extensions {
			pieExtensions[ext] = struct{}{}
			extensionSources[ext] = "pie"
		}
	}

	extensionsToShow := make([]extensionInfo, 0, len(pieExtensions)+len(requiredExtensions))

	for ext := range pieExtensions {
		if ext == "" || ext == "Core" || ext == "standard" || ext == "[PHP Modules]" || ext == "[Zend Modules]" {
			continue
		}

		extensionsToShow = append(extensionsToShow, buildExtensionInfo(ext, "pie"))
	}

	for _, ext := range requiredExtensions {
		if slices.ContainsFunc(extensionsToShow, func(info extensionInfo) bool { return info.Name == ext }) {
			continue
		}

		if source, exists := extensionSources[ext]; exists {
			extensionsToShow = append(extensionsToShow, buildExtensionInfo(ext, source))
			continue
		}

		if semver.MatchVersionConstraint(phpVersion, ">=8.4") {
			if altExt := findPdoAlternative(ext, extensionSources); altExt != "" {
				extensionsToShow = append(extensionsToShow, extensionInfo{
					Name:    ext,
					Source:  "php",
					Warning: fmt.Sprintf("%s(%s)%s", terminal.Gray, altExt, terminal.Reset),
				})
				continue
			}
		}

		messages = append(messages, model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("Missing extension %s%s, Please enable it!", terminal.Reset, ext)})
	}

	slices.SortFunc(extensionsToShow, func(a, b extensionInfo) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	for _, extInfo := range extensionsToShow {
		messages = append(messages, extensionFeedback(extInfo))
	}

	return messages
}

func extensionFeedback(extInfo extensionInfo) model.Message {
	if extInfo.IsWarning {
		return model.Message{Severity: model.SeverityWarning, Text: fmt.Sprintf("Installed extension %s%s %s", terminal.Reset, extInfo.Name, extInfo.Warning), Nested: true}
	}

	if extInfo.Source == "pie" {
		return model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed extension %s%s %s(Pie)%s", terminal.Reset, extInfo.Name, terminal.Yellow, terminal.Reset), Nested: true}
	}

	if extInfo.Warning != "" {
		return model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed extension %s%s %s", terminal.Reset, extInfo.Name, extInfo.Warning), Nested: true}
	}

	return model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed extension %s%s", terminal.Reset, extInfo.Name), Nested: true}
}

func buildExtensionInfo(ext, source string) extensionInfo {
	info := extensionInfo{Name: ext, Source: source}

	if _, deprecated := deprecatedExtensions[ext]; deprecated {
		info.IsWarning = true
		info.Warning = fmt.Sprintf("(%s ⟶ deprecated), Consider removing or replacing it!", ext)
	} else if _, experimental := experimentalExtensions[ext]; experimental {
		info.IsWarning = true
		info.Warning = fmt.Sprintf("(%s ⟶ experimental), Use with caution!", ext)
	}

	return info
}

func findPdoAlternative(ext string, extensionSources map[string]string) string {
	pdoExtensions := map[string][]string{
		"pdo": {"pdo_sqlite", "pdo_mysql", "pdo_pgsql", "pdo_oci", "pdo_odbc", "pdo_firebird"},
	}

	alternatives, isSplitExt := pdoExtensions[ext]

	if !isSplitExt {
		return ""
	}

	for _, altExt := range alternatives {
		if _, exists := extensionSources[altExt]; exists {
			return altExt
		}
	}

	return ""
}

func phpContextFromComposer(rc ecosystem.RunContext) (requiredVersion string, extensions []string, err error) {
	if !rc.FileExists("composer.json") {
		return "", nil, nil
	}

	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "composer.json"))
	if err != nil {
		return "", nil, fmt.Errorf("failed to read composer.json: %w", err)
	}

	var data struct {
		Require map[string]string `json:"require"`
	}

	if err := json.Unmarshal(raw, &data); err != nil {
		return "", nil, fmt.Errorf("failed to parse composer.json: %w", err)
	}

	for dep, version := range data.Require {
		switch {
		case dep == "php":
			requiredVersion = version
		case strings.HasPrefix(dep, "ext-"):
			extensions = append(extensions, strings.TrimPrefix(dep, "ext-"))
		}
	}

	slices.Sort(extensions)

	return requiredVersion, extensions, nil
}

func phpRuntimeVersion(ctx context.Context, rc ecosystem.RunContext) (string, error) {
	result, err := rc.Runner.Run(ctx, "php", "--version")
	if err != nil {
		return "", fmt.Errorf("failed to run php --version: %w", err)
	}

	for line := range strings.SplitSeq(result.Stdout, "\n") {
		if matches := phpVersionRegex.FindStringSubmatch(line); len(matches) >= 2 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("could not parse PHP version from: %s", result.Stdout)
}

func phpExtensions(ctx context.Context, rc ecosystem.RunContext) (map[string]struct{}, error) {
	result, err := rc.Runner.Run(ctx, "php", "-m")
	if err != nil {
		return nil, fmt.Errorf("failed to run php -m: %w", err)
	}

	extensions := make(map[string]struct{})

	for line := range strings.SplitSeq(result.Stdout, "\n") {
		name := strings.TrimSpace(line)

		if name == "" || !moduleNameRegex.MatchString(name) {
			continue
		}

		extensions[name] = struct{}{}
	}

	return extensions, nil
}

type pieConfig struct {
	IsInstalled bool
	Extensions  []string
}

func loadPieConfig(ctx context.Context, rc ecosystem.RunContext) pieConfig {
	invocation := findPieInvocation(ctx, rc)

	if invocation == nil {
		return pieConfig{}
	}

	name := invocation[0]
	args := append(invocation[1:], "show")

	result, err := rc.Runner.Run(ctx, name, args...)
	if err != nil {
		return pieConfig{IsInstalled: true}
	}

	extensions := parsePieShowOutput(result.Stdout)
	slices.Sort(extensions)

	return pieConfig{IsInstalled: true, Extensions: extensions}
}

func findPieInvocation(ctx context.Context, rc ecosystem.RunContext) []string {
	if _, err := rc.Runner.Run(ctx, "pie", "--version"); err == nil {
		return []string{"pie"}
	}

	if rc.FileExists("pie.phar") {
		return []string{"php", "./pie.phar"}
	}

	return nil
}

func parsePieShowOutput(output string) []string {
	var extensions []string

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)

		if line == "" || !strings.Contains(line, "(from ") {
			continue
		}

		name, _, ok := strings.Cut(line, ":")

		if !ok {
			continue
		}

		if name = strings.TrimSpace(name); name != "" {
			extensions = append(extensions, name)
		}
	}

	return extensions
}
