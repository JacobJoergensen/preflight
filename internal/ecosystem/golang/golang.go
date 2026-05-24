package golang

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func Spec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:        "go",
		DisplayName: "Go",
		Priority:    5,
		Managers: []ecosystem.Manager{{
			Command:     "go",
			DisplayName: "Go Modules",
			ConfigFile:  "go.mod",
			LockFile:    "go.sum",
			VersionArgs: []string{"version"},
			InstallArgs: []string{"mod", "tidy"},
			ForceArgs:   []string{"-mod=mod"},
			Outdated: &ecosystem.OutdatedProbe{
				Args:  []string{"list", "-m", "-u", "-json", "all"},
				Parse: parseOutdated,
			},
			Audit: &ecosystem.AuditProbe{
				Tool:            "govulncheck",
				Args:            []string{"-json", "./..."},
				ToolMissingHint: "govulncheck not found on PATH (install: go install golang.org/x/vuln/cmd/govulncheck@latest)",
				Parse:           parseGovulncheckFindings,
			},
		}},
		Check:   check,
		License: scanLicenses,
	}
}

func scanLicenses(ctx context.Context, rc ecosystem.RunContext, _ ecosystem.Detection) ecosystem.LicenseResult {
	return ecosystem.RunLicenseCommand(ctx, rc, "go-licenses", "go-licenses not found on PATH (install: go install github.com/google/go-licenses@latest)", parseGoLicenses, "csv", "./...")
}

func parseGoLicenses(output string) []ecosystem.PackageLicense {
	var packages []ecosystem.PackageLicense

	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		// go-licenses csv emits: import-path, license-url, license-type. The URL
		// never contains a comma, so the name is the first field and the license
		// is the last.
		fields := strings.Split(line, ",")

		if len(fields) < 3 {
			continue
		}

		name := strings.TrimSpace(fields[0])

		if name == "" {
			continue
		}

		packages = append(packages, ecosystem.PackageLicense{
			Name:    name,
			License: strings.TrimSpace(fields[len(fields)-1]),
		})
	}

	ecosystem.SortPackageLicenses(packages)

	return packages
}

func check(ctx context.Context, rc ecosystem.RunContext, _ ecosystem.Detection) []model.Message {
	if ctx.Err() != nil {
		return nil
	}

	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "go.mod"))
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Failed to read go.mod: %v", err)}}
	}

	requiredVersion, modules := parseGoMod(string(raw))

	installedVersion, err := runtimeVersion(ctx, rc)
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Go is not installed or not on PATH: %v", err)}}
	}

	var messages []model.Message

	if requiredVersion != "" {
		satisfied := semver.MatchMinimumVersion(installedVersion, requiredVersion)
		feedback := ecosystem.VersionFeedback("go", "Go", installedVersion, requiredVersion, semver.MajorMinor(installedVersion), satisfied)
		messages = append(messages, feedback...)
	} else {
		messages = append(messages, model.Message{
			Severity: model.SeveritySuccess,
			Text:     fmt.Sprintf("Installed %sGo (%s ⟶ go.mod)", terminal.Reset, installedVersion),
		})
	}

	if _, err := rc.Runner.Run(ctx, "go", "mod", "verify"); err != nil {
		messages = append(messages, model.Message{Severity: model.SeverityError, Text: ecosystem.FormatExecFailure("go mod verify failed", err)})
	}

	installed := installedModules(ctx, rc)

	for _, module := range modules {
		if _, ok := installed[module]; ok {
			messages = append(messages, model.Message{
				Severity: model.SeveritySuccess,
				Text:     fmt.Sprintf("Installed module %s%s", terminal.Reset, module),
				Nested:   true,
			})
		} else {
			messages = append(messages, model.Message{
				Severity: model.SeverityError,
				Text:     fmt.Sprintf("Missing module %s%s, Run `go get %s`", terminal.Reset, module, module),
				Nested:   true,
			})
		}
	}

	return messages
}

func parseGoMod(content string) (goVersion string, modules []string) {
	var insideRequireBlock bool

	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if after, ok := strings.CutPrefix(line, "go "); ok {
			goVersion = strings.TrimSpace(after)
			continue
		}

		if line == "require (" {
			insideRequireBlock = true
			continue
		}

		if insideRequireBlock {
			if line == ")" {
				insideRequireBlock = false
				continue
			}

			if fields := strings.Fields(line); len(fields) >= 2 {
				modules = append(modules, fields[0])
			}

			continue
		}

		if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			if fields := strings.Fields(line); len(fields) >= 3 {
				modules = append(modules, fields[1])
			}
		}
	}

	slices.Sort(modules)

	return goVersion, modules
}

func runtimeVersion(ctx context.Context, rc ecosystem.RunContext) (string, error) {
	result, err := rc.Runner.Run(ctx, "go", "version")
	if err != nil {
		return "", fmt.Errorf("failed to run go version: %w", err)
	}

	parts := strings.Split(result.Stdout, " ")

	if len(parts) >= 3 {
		return strings.TrimPrefix(parts[2], "go"), nil
	}

	return "", fmt.Errorf("unexpected go version format: %s", result.Stdout)
}

func installedModules(ctx context.Context, rc ecosystem.RunContext) map[string]struct{} {
	modules := make(map[string]struct{})

	result, err := rc.Runner.Run(ctx, "go", "list", "-m", "all")
	if err != nil {
		return modules
	}

	for line := range strings.SplitSeq(result.Stdout, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			if fields := strings.Fields(trimmed); len(fields) > 0 {
				modules[fields[0]] = struct{}{}
			}
		}
	}

	return modules
}

func parseOutdated(_ ecosystem.RunContext, output string) ([]ecosystem.OutdatedPackage, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	var packages []ecosystem.OutdatedPackage

	decoder := json.NewDecoder(strings.NewReader(output))

	for decoder.More() {
		var module struct {
			Path     string `json:"Path"`
			Version  string `json:"Version"`
			Indirect bool   `json:"Indirect"`
			Update   *struct {
				Version string `json:"Version"`
			} `json:"Update"`
		}

		if err := decoder.Decode(&module); err != nil {
			continue
		}

		if module.Indirect || module.Update == nil || module.Version == module.Update.Version {
			continue
		}

		packages = append(packages, ecosystem.OutdatedPackage{
			Name:    module.Path,
			Current: module.Version,
			Latest:  module.Update.Version,
		})
	}

	ecosystem.SortOutdated(packages)

	return packages, nil
}

func parseGovulncheckFindings(jsonText string) []model.Finding {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" {
		return nil
	}

	type osvEntry struct {
		ID      string   `json:"id"`
		Aliases []string `json:"aliases"`
		Summary string   `json:"summary"`
	}

	type findingMessage struct {
		OSV          string `json:"osv"`
		FixedVersion string `json:"fixed_version"`
		Trace        []struct {
			Module  string `json:"module"`
			Version string `json:"version"`
		} `json:"trace"`
	}

	osvByID := make(map[string]osvEntry)
	findingByID := make(map[string]model.Finding)

	for line := range strings.SplitSeq(jsonText, "\n") {
		line = strings.TrimSpace(line)

		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var message struct {
			OSV     *osvEntry       `json:"osv"`
			Finding *findingMessage `json:"finding"`
		}

		if err := json.Unmarshal([]byte(line), &message); err != nil {
			continue
		}

		if message.OSV != nil && message.OSV.ID != "" {
			osvByID[message.OSV.ID] = *message.OSV
		}

		if message.Finding == nil || message.Finding.OSV == "" {
			continue
		}

		module := ""
		version := ""

		if len(message.Finding.Trace) > 0 {
			module = message.Finding.Trace[0].Module
			version = message.Finding.Trace[0].Version
		}

		existing, seen := findingByID[message.Finding.OSV]

		// govulncheck emits several finding records per vulnerability; keep the
		// one that names the affected module (the call-stack finding).
		if seen && (existing.Package != "" || module == "") {
			continue
		}

		findingByID[message.Finding.OSV] = model.Finding{
			ID:      message.Finding.OSV,
			Package: module,
			Version: version,
			FixedIn: message.Finding.FixedVersion,
			URL:     "https://pkg.go.dev/vuln/" + message.Finding.OSV,
		}
	}

	if len(findingByID) == 0 {
		return nil
	}

	findings := make([]model.Finding, 0, len(findingByID))

	for id, finding := range findingByID {
		if osv, ok := osvByID[id]; ok {
			finding.Aliases = osv.Aliases
			finding.Summary = osv.Summary
		}

		finding.Severity = "high" // govulncheck reports reachable vulnerabilities without a CVSS score

		findings = append(findings, finding)
	}

	ecosystem.SortFindings(findings)

	return findings
}
