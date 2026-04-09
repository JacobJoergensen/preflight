package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/manifest"
	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func init() {
	Register(PythonModule{})
}

type PythonModule struct{}

func (p PythonModule) Name() string {
	return "python"
}

func (p PythonModule) DisplayName() string {
	return "Python"
}

func (p PythonModule) Check(ctx context.Context, deps Dependencies) ([]Message, []Message, []Message) {
	errs := []Message{}
	warns := []Message{}
	succs := []Message{}

	if ctx.Err() != nil {
		return errs, warns, succs
	}

	config := deps.Loader.LoadPythonConfig()
	packageManager := config.PackageManager

	if packageManager.Tool.Command == "" {
		return errs, warns, succs
	}

	if !packageManager.ConfigFileExists && !packageManager.LockFileExists {
		return errs, warns, succs
	}

	if config.Error != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Failed to read Python project files: %v", config.Error)})
		return errs, warns, succs
	}

	if config.PythonVersionPin != "" && config.RequiresPythonConstraint != "" {
		pin := strings.TrimPrefix(strings.TrimSpace(config.PythonVersionPin), "v")

		if !semver.MatchVersionConstraint(pin, config.RequiresPythonConstraint) {
			warns = append(warns, Message{Text: fmt.Sprintf(
				".python-version pins %s but requires-python is %s — align these for consistent installs.",
				strings.TrimSpace(config.PythonVersionPin),
				config.RequiresPythonConstraint,
			)})
		}
	}

	if config.RequiresPython != "" {
		pythonVersion, err := getPythonVersion(ctx, deps.Runner)

		if err != nil {
			warns = append(warns, Message{Text: fmt.Sprintf("Could not run python --version: %v", err)})
		} else {
			ok, msg := semver.ValidateVersion(pythonVersion, config.RequiresPython)

			if !ok {
				if msg != "" {
					warns = append(warns, Message{Text: msg})
				} else {
					warns = append(warns, Message{Text: fmt.Sprintf(
						"Python version %s does not satisfy requires-python %s",
						pythonVersion,
						config.RequiresPython,
					)})
				}
			}
		}
	}

	installedVersion, err := toolVersion(ctx, deps.Runner, packageManager.Command())

	if err != nil {
		warns = append(warns, Message{Text: fmt.Sprintf("%s not available or not on PATH: %v", packageManager.Name(), err)})
	} else {
		tool, found := manifest.GetTool(packageManager.Command())
		if !found {
			tool = manifest.Tool{Name: packageManager.Name(), Command: packageManager.Command()}
		}

		version := trimFirstLine(installedVersion)
		var rhs string

		switch {
		case packageManager.LockFileExists && packageManager.Tool.LockFile != "":
			rhs = packageManager.Tool.LockFile
		case packageManager.ConfigFileExists && packageManager.Tool.ConfigFile != "":
			rhs = packageManager.Tool.ConfigFile
		}

		if rhs != "" {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed %s%s (%s ⟶ %s)", terminal.Reset, tool.Name, version, rhs)})
		} else {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed %s%s (%s)", terminal.Reset, tool.Name, version)})
		}
	}

	if err := runPythonPipCheck(ctx, deps.Runner, packageManager.Command()); err != nil {
		errs = append(errs, Message{Text: formatExecFailure("pip check failed", err)})
	}

	installed := installedPackagesForPython(ctx, deps.Runner, packageManager.Command())

	seenDep := make(map[string]struct{})

	processDep := func(dep string, isDev bool) {
		if dep == "" {
			return
		}

		key := strings.ToLower(dep)

		if _, dup := seenDep[key]; dup {
			return
		}

		seenDep[key] = struct{}{}

		if version, ok := lookupInstalledLower(installed, dep); ok {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed package %s%s (%s)", terminal.Reset, dep, version), Nested: true, Dev: isDev})
		} else {
			errs = append(errs, Message{Text: fmt.Sprintf(
				"Missing package %s%s, Run your install command (e.g. `%s install`).",
				terminal.Reset,
				dep,
				packageManager.Command(),
			), Nested: true, Dev: isDev})
		}
	}

	for _, dep := range config.Dependencies {
		processDep(dep, false)
	}

	for _, dep := range config.DevDependencies {
		processDep(dep, true)
	}

	return errs, warns, succs
}

func (p PythonModule) ListDependencies(ctx context.Context, deps Dependencies) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadPythonConfig()

	if config.PackageManager.Tool.Command == "" {
		return nil, nil
	}

	if config.Error != nil {
		return nil, config.Error
	}

	if !config.HasConfig {
		return nil, nil
	}

	return append(slices.Clone(config.Dependencies), config.DevDependencies...), nil
}

func (p PythonModule) ListOutdated(ctx context.Context, deps Dependencies) ([]OutdatedPackage, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadPythonConfig()

	if config.PackageManager.Tool.Command == "" {
		return nil, nil
	}

	output, err := runPipListOutdated(ctx, deps.Runner, config.PackageManager.Command())

	if err != nil {
		return nil, err
	}

	packages, err := parsePipOutdated(output)

	if err != nil {
		return nil, err
	}

	direct := toSet(config.Dependencies, config.DevDependencies)

	return filterDirectOutdated(packages, direct), nil
}

func runPipListOutdated(ctx context.Context, runner exec.Runner, pmCommand string) (string, error) {
	args := []string{"-m", "pip", "list", "--outdated", "--format=json"}

	switch pmCommand {
	case "poetry":
		return runner.Run(ctx, "poetry", append([]string{"run", "python"}, args...)...)
	case "uv":
		return runner.Run(ctx, "uv", append([]string{"run", "python"}, args...)...)
	case "pipenv":
		return runner.Run(ctx, "pipenv", append([]string{"run", "python"}, args...)...)
	case "pdm":
		return runner.Run(ctx, "pdm", append([]string{"run", "python"}, args...)...)
	default:
		output, err := runner.Run(ctx, "python", args...)

		if err != nil {
			return runner.Run(ctx, "python3", args...)
		}

		return output, nil
	}
}

func parsePipOutdated(output string) ([]OutdatedPackage, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	var entries []struct {
		Name          string `json:"name"`
		Version       string `json:"version"`
		LatestVersion string `json:"latest_version"`
	}

	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		return nil, err
	}

	packages := make([]OutdatedPackage, 0, len(entries))

	for _, entry := range entries {
		packages = append(packages, OutdatedPackage{
			Name:    entry.Name,
			Current: entry.Version,
			Latest:  entry.LatestVersion,
		})
	}

	slices.SortFunc(packages, func(a, b OutdatedPackage) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return packages, nil
}

func (p PythonModule) Fix(ctx context.Context, deps Dependencies, selectors []string, options FixOptions) (FixItem, error) {
	return fixWithSelectorCheck(ctx, deps, p.Name(), manifest.PackageTypePython, selectors, options)
}

func toolVersion(ctx context.Context, runner exec.Runner, command string) (string, error) {
	tool, ok := manifest.GetTool(command)

	if !ok {
		return "", fmt.Errorf("unknown tool: %s", command)
	}

	output, err := runner.Run(ctx, tool.Command, tool.VersionArgs...)

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}

var pythonSemverFromVersion = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func getPythonVersion(ctx context.Context, runner exec.Runner) (string, error) {
	output, err := runner.Run(ctx, "python", "--version")

	if err != nil {
		output, err = runner.Run(ctx, "python3", "--version")
	}

	if err != nil {
		return "", err
	}

	matches := pythonSemverFromVersion.FindStringSubmatch(output)

	if len(matches) < 2 {
		return "", fmt.Errorf("unexpected python version output: %s", strings.TrimSpace(output))
	}

	return matches[1], nil
}

// runPythonPipCheck runs `python -m pip check` in the same environment as pip list (PEP 517 tools use `tool run python`).
func runPythonPipCheck(ctx context.Context, runner exec.Runner, pmCommand string) error {
	check := []string{"-m", "pip", "check"}

	switch pmCommand {
	case "pip":
		_, err := runner.Run(ctx, "python", check...)

		if err != nil {
			_, err2 := runner.Run(ctx, "python3", check...)

			return err2
		}

		return nil
	case "poetry":
		_, err := runner.Run(ctx, "poetry", append([]string{"run", "python"}, check...)...)

		return err
	case "uv":
		_, err := runner.Run(ctx, "uv", append([]string{"run", "python"}, check...)...)

		return err
	case "pipenv":
		_, err := runner.Run(ctx, "pipenv", append([]string{"run", "python"}, check...)...)

		return err
	case "pdm":
		_, err := runner.Run(ctx, "pdm", append([]string{"run", "python"}, check...)...)

		return err
	default:
		_, err := runner.Run(ctx, "python", check...)

		if err != nil {
			_, err2 := runner.Run(ctx, "python3", check...)

			return err2
		}

		return nil
	}
}

func installedPackagesForPython(ctx context.Context, runner exec.Runner, command string) map[string]string {
	switch command {
	case "pip":
		return pipListMap(ctx, runner, nil)
	case "poetry":
		return pipListMap(ctx, runner, []string{"poetry", "run"})
	case "uv":
		return pipListMap(ctx, runner, []string{"uv", "run"})
	case "pipenv":
		return pipListMap(ctx, runner, []string{"pipenv", "run"})
	case "pdm":
		return pipListMap(ctx, runner, []string{"pdm", "run"})
	default:
		return pipListMap(ctx, runner, nil)
	}
}

func pipListMap(ctx context.Context, runner exec.Runner, prefix []string) map[string]string {
	output, err := runPipListJSON(ctx, runner, prefix)

	if err != nil {
		return map[string]string{}
	}

	var entries []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	if json.Unmarshal([]byte(output), &entries) != nil {
		return map[string]string{}
	}

	packages := make(map[string]string, len(entries))

	for _, entry := range entries {
		if entry.Name == "" {
			continue
		}

		packages[strings.ToLower(entry.Name)] = entry.Version
	}

	return packages
}

func runPipListJSON(ctx context.Context, runner exec.Runner, prefix []string) (string, error) {
	args := []string{"-m", "pip", "list", "--format=json"}

	if len(prefix) == 0 {
		output, err := runner.Run(ctx, "python", args...)

		if err != nil {
			return runner.Run(ctx, "python3", args...)
		}

		return output, nil
	}

	fullArgs := append(append([]string{}, prefix...), append([]string{"python"}, args...)...)

	return runner.Run(ctx, fullArgs[0], fullArgs[1:]...)
}
