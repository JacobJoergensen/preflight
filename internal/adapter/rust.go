package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
	"github.com/JacobJoergensen/preflight/internal/manifest"
	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func init() {
	Register(RustModule{})
}

type RustModule struct{}

func (RustModule) Name() string {
	return "rust"
}

func (RustModule) DisplayName() string {
	return "Rust"
}

func (r RustModule) Check(ctx context.Context, deps Dependencies) ([]Message, []Message, []Message) {
	errs := []Message{}
	warns := []Message{}
	succs := []Message{}

	if ctx.Err() != nil {
		return errs, warns, succs
	}

	cargoConfig := deps.Loader.LoadCargoConfig()

	if !cargoConfig.HasManifest {
		return errs, warns, succs
	}

	if cargoConfig.Error != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Failed to read Cargo.toml: %v", cargoConfig.Error)})
		return errs, warns, succs
	}

	cargoVersion, err := getCargoVersion(ctx, deps.Runner)
	if err != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Cargo is not installed or not on PATH: %v", err)})
		return errs, warns, succs
	}

	if cargoConfig.RustVersion != "" {
		rustcVersion, rustcErr := getRustcVersion(ctx, deps.Runner)

		if rustcErr != nil {
			errs = append(errs, Message{Text: fmt.Sprintf("rustc is not installed or not on PATH: %v", rustcErr)})
			return errs, warns, succs
		}

		versionPrefix := semver.ParseVersionPin(rustcVersion)
		satisfied := semver.MatchMinimumVersion(rustcVersion, cargoConfig.RustVersion)
		feedback := buildVersionFeedback("rust", "Rust", rustcVersion, cargoConfig.RustVersion, versionPrefix, satisfied)
		errs, warns, succs = appendVersionFeedback(feedback, errs, warns, succs)
	} else {
		succs = append(succs, Message{Text: fmt.Sprintf("Installed %sCargo (%s)", terminal.Reset, cargoVersion)})
	}

	succs = append(succs, Message{Text: "Cargo.toml found:"})

	installed := installedFromCargoLock(deps.FS, deps.Loader.WorkDir)

	for _, dep := range cargoConfig.Dependencies {
		if version, ok := installed[dep]; ok {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed crate %s%s (%s)", terminal.Reset, dep, version), Nested: true})
		} else {
			errs = append(errs, Message{Text: fmt.Sprintf("Missing crate %s%s, Run `cargo build`", terminal.Reset, dep), Nested: true})
		}
	}

	for _, dep := range cargoConfig.DevDependencies {
		if version, ok := installed[dep]; ok {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed crate %s%s (%s)", terminal.Reset, dep, version), Nested: true, Dev: true})
		} else {
			errs = append(errs, Message{Text: fmt.Sprintf("Missing crate %s%s, Run `cargo build`", terminal.Reset, dep), Nested: true, Dev: true})
		}
	}

	return errs, warns, succs
}

func (r RustModule) ListOutdated(ctx context.Context, deps Dependencies) ([]OutdatedPackage, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadCargoConfig()

	if !config.HasManifest {
		return nil, nil
	}

	output, err := deps.Runner.Run(ctx, "cargo", "outdated", "--format", "json", "--depth", "1")

	if err != nil && output == "" {
		return nil, err
	}

	packages, err := parseCargoOutdated(output)
	if err != nil {
		return nil, err
	}

	direct := toSet(config.Dependencies, config.DevDependencies, config.OptionalDependencies)

	return filterDirectOutdated(packages, direct), nil
}

func (r RustModule) Fix(ctx context.Context, deps Dependencies, _ []string, options FixOptions) (FixItem, error) {
	return fixByPackageType(ctx, deps, r.Name(), manifest.PackageTypeRust, options)
}

func getCargoVersion(ctx context.Context, runner exec.Runner) (string, error) {
	output, err := runner.Run(ctx, "cargo", "--version")
	if err != nil {
		return "", fmt.Errorf("failed to run cargo --version: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(output))

	if len(fields) >= 2 {
		return fields[1], nil
	}

	return "", fmt.Errorf("unexpected cargo version format: %s", output)
}

func getRustcVersion(ctx context.Context, runner exec.Runner) (string, error) {
	output, err := runner.Run(ctx, "rustc", "--version")
	if err != nil {
		return "", fmt.Errorf("failed to run rustc --version: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(output))

	if len(fields) >= 2 {
		return fields[1], nil
	}

	return "", fmt.Errorf("unexpected rustc version format: %s", output)
}

func installedFromCargoLock(fsys fs.FS, workDir string) map[string]string {
	data, err := fsys.ReadFile(filepath.Join(workDir, "Cargo.lock"))
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

func parseCargoOutdated(output string) ([]OutdatedPackage, error) {
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

	packages := make([]OutdatedPackage, 0, len(report.Dependencies))

	for _, entry := range report.Dependencies {
		if entry.Name == "" || entry.Latest == "" || entry.Latest == "---" {
			continue
		}

		if entry.Project == entry.Latest {
			continue
		}

		packages = append(packages, OutdatedPackage{
			Name:    entry.Name,
			Current: entry.Project,
			Latest:  entry.Latest,
		})
	}

	slices.SortFunc(packages, func(a, b OutdatedPackage) int {
		return strings.Compare(a.Name, b.Name)
	})

	return packages, nil
}
