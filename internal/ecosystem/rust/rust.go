package rust

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func Spec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:            "rust",
		DisplayName:     "Rust",
		Priority:        6,
		RuntimeCommands: []string{"rustc"},
		Managers: []ecosystem.Manager{{
			Command:     "cargo",
			DisplayName: "Cargo",
			ConfigFile:  "Cargo.toml",
			LockFile:    "Cargo.lock",
			VersionArgs: []string{"--version"},
			InstallArgs: []string{"fetch"},
			Outdated: &ecosystem.OutdatedProbe{
				Args:  []string{"outdated", "--format", "json", "--depth", "1"},
				Parse: parseOutdated,
			},
			Audit: &ecosystem.AuditProbe{
				Tool:            "cargo-audit",
				Args:            []string{"audit", "--json"},
				ToolMissingHint: "cargo-audit not found on PATH (install: cargo install cargo-audit)",
				Parse:           parseCargoAuditFindings,
			},
		}},
		Check: check,
	}
}

type cargoConfig struct {
	RustVersion          string
	Dependencies         []string
	DevDependencies      []string
	OptionalDependencies []string
}

func check(ctx context.Context, rc ecosystem.RunContext, detection ecosystem.Detection) []model.Message {
	if ctx.Err() != nil {
		return nil
	}

	config, err := loadConfig(rc)
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Failed to read Cargo.toml: %v", err)}}
	}

	cargoVersion, err := versionField(ctx, rc, "cargo")
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("Cargo is not installed or not on PATH: %v", err)}}
	}

	var messages []model.Message

	if config.RustVersion != "" {
		rustcVersion, rustcErr := versionField(ctx, rc, "rustc")
		if rustcErr != nil {
			return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf("rustc is not installed or not on PATH: %v", rustcErr)}}
		}

		satisfied := semver.MatchMinimumVersion(rustcVersion, config.RustVersion)
		feedback := ecosystem.VersionFeedback("rust", "Rust", rustcVersion, config.RustVersion, semver.ParseVersionPin(rustcVersion), satisfied)
		messages = append(messages, feedback...)
	} else {
		messages = append(messages, model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %sCargo (%s)", terminal.Reset, cargoVersion)})
	}

	hasDependencies := len(config.Dependencies)+len(config.DevDependencies)+len(config.OptionalDependencies) > 0
	messages = append(messages, ecosystem.MissingLockfileWarning(rc, detection.Active, hasDependencies)...)

	installed := installedFromCargoLock(rc)

	for _, dep := range config.Dependencies {
		messages = append(messages, crateMessage(dep, installed, false))
	}

	for _, dep := range config.DevDependencies {
		messages = append(messages, crateMessage(dep, installed, true))
	}

	return messages
}

func crateMessage(dep string, installed map[string]string, dev bool) model.Message {
	if version, ok := installed[dep]; ok {
		return model.Message{
			Severity: model.SeveritySuccess,
			Text:     fmt.Sprintf("Installed crate %s%s (%s)", terminal.Reset, dep, version),
			Nested:   true,
			Dev:      dev,
		}
	}

	return model.Message{
		Severity: model.SeverityError,
		Text:     fmt.Sprintf("Missing crate %s%s, Run `cargo build`", terminal.Reset, dep),
		Nested:   true,
		Dev:      dev,
	}
}

func parseOutdated(rc ecosystem.RunContext, output string) ([]ecosystem.OutdatedPackage, error) {
	packages, err := parseCargoOutdated(output)
	if err != nil {
		return nil, err
	}

	config, _ := loadConfig(rc)
	direct := ecosystem.ToSet(config.Dependencies, config.DevDependencies, config.OptionalDependencies)

	return ecosystem.FilterDirect(packages, direct), nil
}

func parseCargoOutdated(output string) ([]ecosystem.OutdatedPackage, error) {
	trimmed := strings.TrimSpace(output)

	if trimmed == "" {
		return nil, nil
	}

	var report struct {
		Dependencies []struct {
			Name    string `json:"name"`
			Project string `json:"project"`
			Compat  string `json:"compat"`
			Latest  string `json:"latest"`
		} `json:"dependencies"`
	}

	if err := json.Unmarshal([]byte(trimmed), &report); err != nil {
		return nil, err
	}

	packages := make([]ecosystem.OutdatedPackage, 0, len(report.Dependencies))

	for _, entry := range report.Dependencies {
		if entry.Name == "" || entry.Latest == "" || entry.Latest == "---" {
			continue
		}

		if entry.Project == entry.Latest {
			continue
		}

		packages = append(packages, ecosystem.OutdatedPackage{
			Name:    entry.Name,
			Current: entry.Project,
			Latest:  entry.Latest,
		})
	}

	ecosystem.SortOutdated(packages)

	return packages, nil
}

func versionField(ctx context.Context, rc ecosystem.RunContext, command string) (string, error) {
	result, err := rc.Runner.Run(ctx, command, "--version")
	if err != nil {
		return "", fmt.Errorf("failed to run %s --version: %w", command, err)
	}

	fields := strings.Fields(result.Stdout)

	if len(fields) >= 2 {
		return fields[1], nil
	}

	return "", fmt.Errorf("unexpected %s version format: %s", command, result.Stdout)
}

func loadConfig(rc ecosystem.RunContext) (cargoConfig, error) {
	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "Cargo.toml"))
	if err != nil {
		return cargoConfig{}, err
	}

	return parseCargoToml(raw), nil
}

func installedFromCargoLock(rc ecosystem.RunContext) map[string]string {
	data, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, "Cargo.lock"))
	if err != nil {
		return map[string]string{}
	}

	parser, ok := lockdiff.ParserFor("Cargo.lock")
	if !ok {
		return map[string]string{}
	}

	parsed, err := parser.Parse(data)
	if err != nil {
		return map[string]string{}
	}

	return parsed
}

func parseCargoToml(raw []byte) cargoConfig {
	var doc struct {
		Package struct {
			RustVersion string `toml:"rust-version"`
		} `toml:"package"`
		Dependencies    map[string]any `toml:"dependencies"`
		DevDependencies map[string]any `toml:"dev-dependencies"`
	}

	if err := toml.Unmarshal(raw, &doc); err != nil {
		return cargoConfig{}
	}

	config := cargoConfig{RustVersion: doc.Package.RustVersion}

	for name, value := range doc.Dependencies {
		if cargoDepOptional(value) {
			config.OptionalDependencies = append(config.OptionalDependencies, name)
		} else {
			config.Dependencies = append(config.Dependencies, name)
		}
	}

	for name := range doc.DevDependencies {
		config.DevDependencies = append(config.DevDependencies, name)
	}

	slices.Sort(config.Dependencies)
	slices.Sort(config.DevDependencies)
	slices.Sort(config.OptionalDependencies)

	return config
}

func cargoDepOptional(value any) bool {
	table, ok := value.(map[string]any)
	if !ok {
		return false
	}

	optional, _ := table["optional"].(bool)

	return optional
}

func parseCargoAuditFindings(jsonText string) []model.Finding {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" {
		return nil
	}

	var report struct {
		Vulnerabilities struct {
			List []struct {
				Advisory struct {
					ID            string   `json:"id"`
					Package       string   `json:"package"`
					Title         string   `json:"title"`
					URL           string   `json:"url"`
					Aliases       []string `json:"aliases"`
					Informational string   `json:"informational"`
					CVSS          string   `json:"cvss"`
				} `json:"advisory"`
				Versions struct {
					Patched []string `json:"patched"`
				} `json:"versions"`
				Package struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"package"`
			} `json:"list"`
		} `json:"vulnerabilities"`
	}

	if err := json.Unmarshal([]byte(jsonText), &report); err != nil {
		return nil
	}

	findings := make([]model.Finding, 0, len(report.Vulnerabilities.List))

	for _, vuln := range report.Vulnerabilities.List {
		pkg := vuln.Advisory.Package
		if pkg == "" {
			pkg = vuln.Package.Name
		}

		fixedIn := ""
		if len(vuln.Versions.Patched) > 0 {
			fixedIn = vuln.Versions.Patched[0]
		}

		findings = append(findings, model.Finding{
			ID:       vuln.Advisory.ID,
			Aliases:  vuln.Advisory.Aliases,
			Severity: advisorySeverity(vuln.Advisory.Informational, vuln.Advisory.CVSS),
			Package:  pkg,
			Version:  vuln.Package.Version,
			FixedIn:  fixedIn,
			URL:      vuln.Advisory.URL,
			Summary:  vuln.Advisory.Title,
		})
	}

	if len(findings) == 0 {
		return nil
	}

	ecosystem.SortFindings(findings)

	return findings
}

func advisorySeverity(informational, cvss string) string {
	if informational != "" || cvss == "" {
		return "info"
	}

	score, ok := cvssBaseScore(cvss)

	if !ok {
		return "info"
	}

	switch {
	case score >= 9.0:
		return "critical"
	case score >= 7.0:
		return "high"
	case score >= 4.0:
		return "moderate"
	case score > 0:
		return "low"
	default:
		return "info"
	}
}

var (
	cvssAttackVector                = map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2}
	cvssAttackComplexity            = map[string]float64{"L": 0.77, "H": 0.44}
	cvssUserInteraction             = map[string]float64{"N": 0.85, "R": 0.62}
	cvssImpact                      = map[string]float64{"N": 0, "L": 0.22, "H": 0.56}
	cvssPrivilegesRequiredUnchanged = map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27}
	cvssPrivilegesRequiredChanged   = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.50}
)

func cvssBaseScore(vector string) (float64, bool) {
	metrics, ok := parseCVSSVector(vector)

	if !ok {
		return 0, false
	}

	avVal, ok := cvssAttackVector[metrics["AV"]]

	if !ok {
		return 0, false
	}

	acVal, ok := cvssAttackComplexity[metrics["AC"]]

	if !ok {
		return 0, false
	}

	uiVal, ok := cvssUserInteraction[metrics["UI"]]

	if !ok {
		return 0, false
	}

	scope := metrics["S"]

	var prVal float64

	if scope == "C" {
		prVal, ok = cvssPrivilegesRequiredChanged[metrics["PR"]]
	} else {
		prVal, ok = cvssPrivilegesRequiredUnchanged[metrics["PR"]]
	}

	if !ok {
		return 0, false
	}

	cVal, ok := cvssImpact[metrics["C"]]

	if !ok {
		return 0, false
	}

	iVal, ok := cvssImpact[metrics["I"]]

	if !ok {
		return 0, false
	}

	aVal, ok := cvssImpact[metrics["A"]]

	if !ok {
		return 0, false
	}

	iss := 1 - ((1 - cVal) * (1 - iVal) * (1 - aVal))

	var impact float64

	if scope == "C" {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	} else {
		impact = 6.42 * iss
	}

	if impact <= 0 {
		return 0, true
	}

	exploitability := 8.22 * avVal * acVal * prVal * uiVal

	var baseScore float64

	if scope == "C" {
		baseScore = math.Min(1.08*(impact+exploitability), 10)
	} else {
		baseScore = math.Min(impact+exploitability, 10)
	}

	return cvssRoundUp(baseScore), true
}

func parseCVSSVector(vector string) (map[string]string, bool) {
	parts := strings.Split(vector, "/")

	if len(parts) < 2 {
		return nil, false
	}

	if parts[0] != "CVSS:3.0" && parts[0] != "CVSS:3.1" {
		return nil, false
	}

	metrics := make(map[string]string)

	for _, part := range parts[1:] {
		key, value, found := strings.Cut(part, ":")

		if !found {
			continue
		}

		metrics[key] = value
	}

	return metrics, true
}

func cvssRoundUp(value float64) float64 {
	intInput := int(math.Round(value * 100000))

	if intInput%10000 == 0 {
		return float64(intInput) / 100000.0
	}

	return float64(intInput/10000+1) / 10.0
}
