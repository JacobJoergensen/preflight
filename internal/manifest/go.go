package manifest

import (
	"fmt"
	"slices"
	"strings"
)

type GoConfig struct {
	PackageManager PackageManager
	GoVersion      string
	Modules        []string
	HasMod         bool
	Error          error
}

func (l Loader) LoadGoConfig() GoConfig {
	config := GoConfig{}
	config.PackageManager, _ = l.DetectPackageManager("go")
	config.HasMod = config.PackageManager.ConfigFileExists

	if !config.HasMod {
		return config
	}

	raw, err := l.readFile("go.mod")

	if err != nil {
		config.Error = fmt.Errorf("failed to read go.mod: %w", err)
		return config
	}

	parseGoMod(&config, string(raw))

	return config
}

func parseGoMod(config *GoConfig, content string) {
	lines := strings.Split(content, "\n")
	var insideRequireBlock bool

	config.Modules = make([]string, 0, len(lines)/2)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if after, ok := strings.CutPrefix(line, "go "); ok {
			config.GoVersion = strings.TrimSpace(after)
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
				config.Modules = append(config.Modules, fields[0])
			}

			continue
		}

		if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			if fields := strings.Fields(line); len(fields) >= 3 {
				config.Modules = append(config.Modules, fields[1])
			}
		}
	}

	slices.Sort(config.Modules)
}
