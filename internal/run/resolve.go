package run

import (
	"errors"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/js"
	"github.com/JacobJoergensen/preflight/internal/ecosystem/python"
)

type ResolvedScript struct {
	Bin  string
	Args []string
}

func ResolveScripts(rc ecosystem.RunContext, targets config.ScriptTargets) ([]ResolvedScript, error) {
	resolved := make([]ResolvedScript, 0, len(targets))

	for _, target := range targets {
		bin, args, err := ResolveScript(rc, target)
		if err != nil {
			return nil, err
		}

		resolved = append(resolved, ResolvedScript{Bin: bin, Args: args})
	}

	return resolved, nil
}

func ResolveScript(rc ecosystem.RunContext, target config.ScriptTarget) (bin string, args []string, err error) {
	if err := target.Validate(); err != nil {
		return "", nil, err
	}

	switch {
	case target.JS != "":
		detection, ok := js.Spec().Resolve(rc)

		if !ok {
			return "", nil, errors.New("no JavaScript package manager detected for js script")
		}

		return detection.Active.Command, []string{"run", target.JS}, nil

	case target.Composer != "":
		return "composer", []string{"run-script", target.Composer}, nil

	case target.Go != "":
		parts := strings.Fields(target.Go)

		if len(parts) == 0 {
			return "", nil, errors.New("go script value is empty")
		}

		if parts[0] != "go" {
			return "go", parts, nil
		}

		return "go", parts[1:], nil

	case target.Ruby != "":
		parts := strings.Fields(target.Ruby)

		if len(parts) == 0 {
			return "", nil, errors.New("ruby script value is empty")
		}

		return "bundle", append([]string{"exec"}, parts...), nil

	case target.Python != "":
		detection, ok := python.Spec().Resolve(rc)

		if !ok {
			return "", nil, errors.New("no Python project detected for python script")
		}

		parts := strings.Fields(target.Python)

		if len(parts) == 0 {
			return "", nil, errors.New("python script value is empty")
		}

		switch detection.Active.Command {
		case "poetry", "uv", "pipenv", "pdm":
			return detection.Active.Command, append([]string{"run"}, parts...), nil
		default:
			return "python", parts, nil
		}

	case target.Rust != "":
		parts := strings.Fields(target.Rust)

		if len(parts) == 0 {
			return "", nil, errors.New("rust script value is empty")
		}

		if parts[0] == "cargo" {
			return "cargo", parts[1:], nil
		}

		return "cargo", parts, nil

	case target.Dotnet != "":
		parts := strings.Fields(target.Dotnet)

		if len(parts) == 0 {
			return "", nil, errors.New("dotnet script value is empty")
		}

		if parts[0] == "dotnet" {
			return "dotnet", parts[1:], nil
		}

		return "dotnet", parts, nil
	}

	return "", nil, errors.New("internal: script target did not resolve")
}
