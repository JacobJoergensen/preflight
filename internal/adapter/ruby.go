package adapter

import (
	"context"
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
	Register(RubyModule{})
}

type RubyModule struct{}

func (RubyModule) Name() string {
	return "ruby"
}

func (RubyModule) DisplayName() string {
	return "Ruby"
}

func (r RubyModule) Check(ctx context.Context, deps Dependencies) ([]Message, []Message, []Message) {
	errs := []Message{}
	warns := []Message{}
	succs := []Message{}

	if ctx.Err() != nil {
		return errs, warns, succs
	}

	config := deps.Loader.LoadRubyConfig()
	packageManager := config.PackageManager

	if packageManager.Tool.Command == "" {
		return errs, warns, succs
	}

	if !packageManager.ConfigFileExists && !packageManager.LockFileExists {
		return errs, warns, succs
	}

	if config.Error != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Failed to read Ruby project files: %v", config.Error)})

		return errs, warns, succs
	}

	if config.RubyVersionPin != "" && config.RequiresRubyFromGemfile != "" {
		pinCore := semver.ParseVersionPin(config.RubyVersionPin)

		if pinCore != "" && !semver.MatchVersionConstraint(pinCore, config.RequiresRubyFromGemfile) {
			warns = append(warns, Message{Text: fmt.Sprintf(
				".ruby-version pins %s but Gemfile specifies %s — align these for consistent installs.",
				strings.TrimSpace(config.RubyVersionPin),
				strings.TrimSpace(config.RequiresRubyFromGemfile),
			)})
		}
	}

	if config.RequiresRuby != "" {
		rubyVersion, err := getRubyVersion(ctx, deps.Runner)

		if err != nil {
			warns = append(warns, Message{Text: fmt.Sprintf("Could not read Ruby version: %v", err)})
		} else {
			ok, msg := semver.ValidateVersion(rubyVersion, config.RequiresRuby)

			if !ok {
				if msg != "" {
					warns = append(warns, Message{Text: msg})
				} else {
					warns = append(warns, Message{Text: fmt.Sprintf(
						"Ruby version %s does not satisfy %s",
						rubyVersion,
						config.RequiresRuby,
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
		rhs := "Gemfile.lock"

		if packageManager.LockFileExists && packageManager.Tool.LockFile != "" {
			rhs = packageManager.Tool.LockFile
		}

		succs = append(succs, Message{Text: fmt.Sprintf("Installed %s%s (%s ⟶ %s)", terminal.Reset, tool.Name, version, rhs)})
	}

	if packageManager.LockFileExists && packageManager.Tool.LockFile != "" {
		if _, err := deps.Runner.Run(ctx, "bundle", "check"); err != nil {
			errs = append(errs, Message{Text: formatExecFailure("bundle check failed", err)})
		}
	}

	installed := installedGemsFromBundle(ctx, deps.Runner)
	seenDep := make(map[string]struct{})

	for _, dep := range config.Dependencies {
		if dep == "" {
			continue
		}

		key := strings.ToLower(dep)

		if _, dup := seenDep[key]; dup {
			continue
		}

		seenDep[key] = struct{}{}

		if version, ok := lookupInstalledLower(installed, dep); ok {
			succs = append(succs, Message{Text: fmt.Sprintf("Installed gem %s%s (%s)", terminal.Reset, dep, version), Nested: true})
		} else {
			errs = append(errs, Message{Text: fmt.Sprintf(
				"Missing gem %s%s, run `%s install`",
				terminal.Reset,
				dep,
				packageManager.Command(),
			), Nested: true})
		}
	}

	return errs, warns, succs
}

func (r RubyModule) ListDependencies(ctx context.Context, deps Dependencies) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	config := deps.Loader.LoadRubyConfig()

	if config.PackageManager.Tool.Command == "" {
		return nil, nil
	}

	if config.Error != nil {
		return nil, config.Error
	}

	if !config.HasConfig {
		return nil, nil
	}

	return slices.Clone(config.Dependencies), nil
}

func (r RubyModule) Fix(ctx context.Context, deps Dependencies, selectors []string, options FixOptions) (FixItem, error) {
	return fixWithSelectorCheck(ctx, deps, r.Name(), manifest.PackageTypeRuby, selectors, options)
}

var bundleListLine = regexp.MustCompile(`^\s*\*\s+([a-zA-Z0-9_.-]+)\s+\(([^)]+)\)`)

func getRubyVersion(ctx context.Context, runner exec.Runner) (string, error) {
	return toolVersion(ctx, runner, "ruby")
}

func installedGemsFromBundle(ctx context.Context, runner exec.Runner) map[string]string {
	output, err := runner.Run(ctx, "bundle", "list")

	if err != nil {
		return map[string]string{}
	}

	return parseBundleListOutput(output)
}

func parseBundleListOutput(output string) map[string]string {
	gems := make(map[string]string)

	for line := range strings.SplitSeq(output, "\n") {
		parts := bundleListLine.FindStringSubmatch(line)

		if len(parts) < 3 {
			continue
		}

		name := strings.ToLower(strings.TrimSpace(parts[1]))
		version := strings.TrimSpace(parts[2])
		gems[name] = version
	}

	return gems
}
