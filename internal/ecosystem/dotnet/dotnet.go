package dotnet

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func Spec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:        "dotnet",
		DisplayName: ".NET",
		Priority:    9,
		Managers: []ecosystem.Manager{{
			Command:     "dotnet",
			DisplayName: ".NET CLI",
			LockFile:    "packages.lock.json",
			VersionArgs: []string{"--version"},
			InstallArgs: []string{"restore"},
			ForceArgs:   []string{"--force"},
			Outdated: &ecosystem.OutdatedProbe{
				Args:  []string{"list", "package", "--outdated", "--format", "json"},
				Parse: parseOutdated,
			},
			Audit: &ecosystem.AuditProbe{
				Args:  []string{"list", "package", "--vulnerable", "--include-transitive", "--format", "json"},
				Parse: parseDotnetVulnerabilities,
			},
		}},
		Detect: []ecosystem.Marker{
			{Glob: "*.csproj", Manager: "dotnet"},
			{Glob: "*.fsproj", Manager: "dotnet"},
			{Glob: "*.vbproj", Manager: "dotnet"},
			{Glob: "*.sln", Manager: "dotnet"},
		},
		Check: check,
	}
}

func check(ctx context.Context, rc ecosystem.RunContext, detection ecosystem.Detection) []model.Message {
	if ctx.Err() != nil {
		return nil
	}

	manager := detection.Active

	installedVersion, err := ecosystem.ToolVersion(ctx, rc, manager)
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: fmt.Sprintf(".NET SDK is not installed or not on PATH: %v", err)}}
	}

	var messages []model.Message

	if manager.LockFile != "" && rc.FileExists(manager.LockFile) {
		messages = append(
			messages,
			model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %s.NET SDK (%s ⟶ %s)", terminal.Reset, ecosystem.FirstLine(installedVersion), manager.LockFile)},
		)
	} else {
		messages = append(messages, model.Message{Severity: model.SeveritySuccess, Text: fmt.Sprintf("Installed %s.NET SDK (%s)", terminal.Reset, ecosystem.FirstLine(installedVersion))})
	}

	packages, err := listPackages(ctx, rc)
	if err != nil {
		messages = append(messages, model.Message{Severity: model.SeverityError, Text: "Project is not restored, run `dotnet restore`"})
		return messages
	}

	for _, pkg := range packages {
		messages = append(messages, model.Message{
			Severity: model.SeveritySuccess,
			Text:     fmt.Sprintf("Installed package %s%s (%s)", terminal.Reset, pkg.id, pkg.version),
			Nested:   true,
		})
	}

	return messages
}

type dotnetPackage struct {
	id      string
	version string
}

func listPackages(ctx context.Context, rc ecosystem.RunContext) ([]dotnetPackage, error) {
	result, err := rc.Runner.Run(ctx, "dotnet", "list", "package", "--format", "json")
	if err != nil {
		return nil, err
	}

	report, ok := parseDotnetReport(result.Stdout)
	if !ok {
		return nil, nil
	}

	var packages []dotnetPackage

	seen := make(map[string]struct{})

	for _, project := range report.Projects {
		for _, framework := range project.Frameworks {
			for _, pkg := range framework.TopLevelPackages {
				key := strings.ToLower(pkg.ID)

				if _, dup := seen[key]; dup {
					continue
				}

				seen[key] = struct{}{}
				packages = append(packages, dotnetPackage{id: pkg.ID, version: pkg.ResolvedVersion})
			}
		}
	}

	slices.SortFunc(packages, func(a, b dotnetPackage) int {
		return strings.Compare(strings.ToLower(a.id), strings.ToLower(b.id))
	})

	return packages, nil
}

func parseDotnetVulnerabilities(jsonText string) []model.Finding {
	report, ok := parseDotnetReport(jsonText)
	if !ok {
		return nil
	}

	var findings []model.Finding

	seen := make(map[string]struct{})

	for _, project := range report.Projects {
		for _, framework := range project.Frameworks {
			packages := make([]dotnetPackageJSON, 0, len(framework.TopLevelPackages)+len(framework.TransitivePackages))
			packages = append(packages, framework.TopLevelPackages...)
			packages = append(packages, framework.TransitivePackages...)

			for _, pkg := range packages {
				for _, vuln := range pkg.Vulnerabilities {
					id := ghsaFromURL(vuln.AdvisoryURL)

					key := id + "|" + strings.ToLower(pkg.ID)
					if id == "" {
						key = vuln.AdvisoryURL + "|" + strings.ToLower(pkg.ID)
					}

					if _, dup := seen[key]; dup {
						continue
					}

					seen[key] = struct{}{}

					findings = append(findings, model.Finding{
						ID:       id,
						Severity: ecosystem.NormalizeSeverity(vuln.Severity),
						Package:  pkg.ID,
						Version:  pkg.ResolvedVersion,
						URL:      vuln.AdvisoryURL,
					})
				}
			}
		}
	}

	ecosystem.SortFindings(findings)

	return findings
}

func parseOutdated(_ ecosystem.RunContext, jsonText string) ([]ecosystem.OutdatedPackage, error) {
	report, ok := parseDotnetReport(jsonText)
	if !ok {
		return nil, nil
	}

	var packages []ecosystem.OutdatedPackage

	seen := make(map[string]struct{})

	for _, project := range report.Projects {
		for _, framework := range project.Frameworks {
			for _, pkg := range framework.TopLevelPackages {
				if pkg.LatestVersion == "" || pkg.LatestVersion == pkg.ResolvedVersion {
					continue
				}

				key := strings.ToLower(pkg.ID)

				if _, dup := seen[key]; dup {
					continue
				}

				seen[key] = struct{}{}
				packages = append(packages, ecosystem.OutdatedPackage{Name: pkg.ID, Current: pkg.ResolvedVersion, Latest: pkg.LatestVersion})
			}
		}
	}

	ecosystem.SortOutdated(packages)

	return packages, nil
}

type dotnetReport struct {
	Projects []struct {
		Frameworks []struct {
			TopLevelPackages   []dotnetPackageJSON `json:"topLevelPackages"`
			TransitivePackages []dotnetPackageJSON `json:"transitivePackages"`
		} `json:"frameworks"`
	} `json:"projects"`
}

type dotnetPackageJSON struct {
	ID              string `json:"id"`
	ResolvedVersion string `json:"resolvedVersion"`
	LatestVersion   string `json:"latestVersion"`
	Vulnerabilities []struct {
		Severity    string `json:"severity"`
		AdvisoryURL string `json:"advisoryurl"`
	} `json:"vulnerabilities"`
}

func parseDotnetReport(jsonText string) (dotnetReport, bool) {
	jsonText = strings.TrimSpace(jsonText)

	if jsonText == "" || !strings.HasPrefix(jsonText, "{") {
		return dotnetReport{}, false
	}

	var report dotnetReport

	if err := json.Unmarshal([]byte(jsonText), &report); err != nil {
		return dotnetReport{}, false
	}

	return report, true
}

func ghsaFromURL(advisoryURL string) string {
	const marker = "/advisories/"

	if i := strings.LastIndex(advisoryURL, marker); i >= 0 {
		return advisoryURL[i+len(marker):]
	}

	return ""
}
