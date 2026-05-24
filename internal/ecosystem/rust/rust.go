package rust

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"

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
				Parse:           parseCargoAuditCounts,
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

	return parseCargoToml(string(raw)), nil
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

func parseCargoToml(content string) cargoConfig {
	var config cargoConfig

	section := ""

	for rawLine := range strings.SplitSeq(content, "\n") {
		line := strings.TrimSpace(stripCargoComment(rawLine))

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		key, value, ok := splitCargoKeyValue(line)
		if !ok {
			continue
		}

		switch section {
		case "package":
			if key == "rust-version" {
				config.RustVersion = unquoteCargoValue(value)
			}
		case "dependencies":
			appendCargoDep(&config, key, value, false)
		case "dev-dependencies":
			appendCargoDep(&config, key, value, true)
		}
	}

	slices.Sort(config.Dependencies)
	slices.Sort(config.DevDependencies)
	slices.Sort(config.OptionalDependencies)

	return config
}

func appendCargoDep(config *cargoConfig, name, value string, isDev bool) {
	if name == "" {
		return
	}

	if !isDev && cargoValueIsOptional(value) {
		config.OptionalDependencies = append(config.OptionalDependencies, name)
		return
	}

	if isDev {
		config.DevDependencies = append(config.DevDependencies, name)
	} else {
		config.Dependencies = append(config.Dependencies, name)
	}
}

func cargoValueIsOptional(value string) bool {
	trimmed := strings.TrimSpace(value)

	if !strings.HasPrefix(trimmed, "{") {
		return false
	}

	return strings.Contains(trimmed, "optional = true") || strings.Contains(trimmed, "optional=true")
}

func splitCargoKeyValue(line string) (key, value string, ok bool) {
	eq := strings.Index(line, "=")

	if eq <= 0 {
		return "", "", false
	}

	key = strings.Trim(strings.TrimSpace(line[:eq]), `"`)
	value = strings.TrimSpace(line[eq+1:])

	if key == "" {
		return "", "", false
	}

	return key, value, true
}

func unquoteCargoValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"'`)
}

func stripCargoComment(line string) string {
	inString := false

	for i, r := range line {
		switch r {
		case '"':
			inString = !inString
		case '#':
			if !inString {
				return line[:i]
			}
		}
	}

	return line
}

func parseCargoAuditCounts(jsonText string) map[string]int {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" {
		return nil
	}

	var report struct {
		Vulnerabilities struct {
			List []struct {
				Advisory struct {
					Informational string `json:"informational"`
					CVSS          string `json:"cvss"`
				} `json:"advisory"`
			} `json:"list"`
		} `json:"vulnerabilities"`
	}

	if err := json.Unmarshal([]byte(jsonText), &report); err != nil {
		return nil
	}

	counts := make(map[string]int)

	for _, vuln := range report.Vulnerabilities.List {
		counts[advisorySeverity(vuln.Advisory.Informational, vuln.Advisory.CVSS)]++
	}

	if len(counts) == 0 {
		return nil
	}

	return counts
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
