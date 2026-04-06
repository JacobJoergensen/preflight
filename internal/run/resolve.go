package run

import (
	"errors"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/config"
	"github.com/JacobJoergensen/preflight/internal/manifest"
)

func ResolveScript(l manifest.Loader, target config.ScriptTarget) (bin string, args []string, err error) {
	if err := target.Validate(); err != nil {
		return "", nil, err
	}

	switch {
	case target.JS != "":
		packageManager, ok := l.DetectPackageManager(manifest.PackageTypeJS)

		if !ok {
			return "", nil, errors.New("no JavaScript package manager detected for js script")
		}

		return packageManager.Command(), []string{"run", target.JS}, nil

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
		packageManager, ok := l.DetectPackageManager(manifest.PackageTypePython)

		if !ok {
			return "", nil, errors.New("no Python project detected for python script")
		}

		parts := strings.Fields(target.Python)

		if len(parts) == 0 {
			return "", nil, errors.New("python script value is empty")
		}

		cmd := packageManager.Command()

		switch cmd {
		case "poetry", "uv", "pipenv", "pdm":
			return cmd, append([]string{"run"}, parts...), nil
		default:
			return "python", parts, nil
		}
	}

	return "", nil, errors.New("internal: script target did not resolve")
}
