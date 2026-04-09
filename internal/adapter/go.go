package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/manifest"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func init() {
	Register(GoModule{})
}

type GoModule struct{}

func (g GoModule) Name() string {
	return "go"
}

func (g GoModule) DisplayName() string {
	return "Go"
}

func (g GoModule) Check(ctx context.Context, deps Dependencies) ([]Message, []Message, []Message) {
	errs := []Message{}
	warns := []Message{}
	succs := []Message{}

	if ctx.Err() != nil {
		return errs, warns, succs
	}

	goConfig := deps.Loader.LoadGoConfig()

	if !goConfig.HasMod {
		return errs, warns, succs
	}

	if goConfig.Error != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Failed to read go.mod: %v", goConfig.Error)})
		return errs, warns, succs
	}

	goVersion, err := getGoVersion(ctx, deps.Runner)

	if err != nil {
		errs = append(errs, Message{Text: "Go is not installed or not available in path."})
		return errs, warns, succs
	}

	if goConfig.GoVersion != "" {
		versionPrefix := strings.Split(goVersion, ".")[0] + "." + strings.Split(goVersion, ".")[1]
		feedback := buildGoVersionFeedback(goVersion, goConfig.GoVersion, versionPrefix)

		if feedback.ShouldWarnEOL {
			warns = append(warns, Message{Text: feedback.EOLWarning})

			if feedback.ShouldWarnExtra {
				warns = append(warns, Message{Text: feedback.Feedback})
			}
		} else if feedback.ShouldError {
			errs = append(errs, Message{Text: feedback.Feedback})
		} else if feedback.ShouldSuccess {
			succs = append(succs, Message{Text: feedback.Feedback})
		}
	} else {
		succs = append(succs, Message{Text: fmt.Sprintf("Installed %sGo (%s ⟶ go.mod)", terminal.Reset, goVersion)})
	}

	if _, err := deps.Runner.Run(ctx, "go", "mod", "verify"); err != nil {
		errs = append(errs, Message{Text: formatGoModVerifyFailure(err)})
	}

	installedModules := getInstalledModules(ctx, deps.Runner)

	for _, mod := range goConfig.Modules {
		if _, exists := installedModules[mod]; exists {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed module %s%s", terminal.Reset, mod), Nested: true})
		} else {
			errs = append(errs, Message{Text: fmt.Sprintf("Missing module %s%s, Run `go get %s`", terminal.Reset, mod, mod), Nested: true})
		}
	}

	return errs, warns, succs
}

func (g GoModule) ListDependencies(ctx context.Context, deps Dependencies) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadGoConfig()

	if !config.HasMod || config.Error != nil {
		return nil, config.Error
	}

	return slices.Clone(config.Modules), nil
}

func (g GoModule) ListOutdated(ctx context.Context, deps Dependencies) ([]OutdatedPackage, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadGoConfig()

	if !config.HasMod {
		return nil, nil
	}

	output, err := deps.Runner.Run(ctx, "go", "list", "-m", "-u", "-json", "all")

	if err != nil && output == "" {
		return nil, err
	}

	return parseGoListOutdated(output)
}

func parseGoListOutdated(output string) ([]OutdatedPackage, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	var packages []OutdatedPackage

	decoder := json.NewDecoder(strings.NewReader(output))

	for decoder.More() {
		var mod struct {
			Path     string `json:"Path"`
			Version  string `json:"Version"`
			Indirect bool   `json:"Indirect"`
			Update   *struct {
				Version string `json:"Version"`
			} `json:"Update"`
		}

		if err := decoder.Decode(&mod); err != nil {
			continue
		}

		if mod.Indirect || mod.Update == nil || mod.Version == mod.Update.Version {
			continue
		}

		packages = append(packages, OutdatedPackage{
			Name:    mod.Path,
			Current: mod.Version,
			Latest:  mod.Update.Version,
		})
	}

	slices.SortFunc(packages, func(a, b OutdatedPackage) int {
		return strings.Compare(a.Name, b.Name)
	})

	return packages, nil
}

func (g GoModule) Fix(ctx context.Context, deps Dependencies, _ []string, options FixOptions) (FixItem, error) {
	return fixByPackageType(ctx, deps, g.Name(), manifest.PackageTypeGo, options)
}

func formatGoModVerifyFailure(err error) string {
	return formatExecFailure("go mod verify failed", err)
}

func getGoVersion(ctx context.Context, runner exec.Runner) (string, error) {
	output, err := runner.Run(ctx, "go", "version")

	if err != nil {
		return "", fmt.Errorf("failed to run go version: %w", err)
	}

	versionOutput := strings.TrimSpace(output)
	parts := strings.Split(versionOutput, " ")

	if len(parts) >= 3 {
		return strings.TrimPrefix(parts[2], "go"), nil
	}

	return "", fmt.Errorf("unexpected go version format: %s", versionOutput)
}

func getInstalledModules(ctx context.Context, runner exec.Runner) map[string]struct{} {
	modules := make(map[string]struct{})

	output, err := runner.Run(ctx, "go", "list", "-m", "all")

	if err != nil {
		return modules
	}

	for line := range strings.SplitSeq(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			fields := strings.Fields(trimmed)

			if len(fields) > 0 {
				modules[fields[0]] = struct{}{}
			}
		}
	}

	return modules
}
