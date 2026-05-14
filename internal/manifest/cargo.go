package manifest

import (
	"fmt"
	"slices"
	"strings"
)

type CargoConfig struct {
	PackageManager       PackageManager
	RustVersion          string
	Dependencies         []string
	DevDependencies      []string
	OptionalDependencies []string
	HasManifest          bool
	Error                error
}

func (l Loader) LoadCargoConfig() CargoConfig {
	config := CargoConfig{}
	config.PackageManager, _ = l.DetectPackageManager(PackageTypeRust)
	config.HasManifest = config.PackageManager.ConfigFileExists

	if !config.HasManifest {
		return config
	}

	raw, err := l.readFile("Cargo.toml")

	if err != nil {
		config.Error = fmt.Errorf("failed to read Cargo.toml: %w", err)
		return config
	}

	parseCargoToml(&config, string(raw))

	return config
}

func parseCargoToml(config *CargoConfig, content string) {
	section := ""

	for rawLine := range strings.SplitSeq(content, "\n") {
		line := stripCargoComment(rawLine)
		line = strings.TrimSpace(line)

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
			appendCargoDep(config, key, value, false)
		case "dev-dependencies":
			appendCargoDep(config, key, value, true)
		}
	}

	slices.Sort(config.Dependencies)
	slices.Sort(config.DevDependencies)
	slices.Sort(config.OptionalDependencies)
}

func appendCargoDep(config *CargoConfig, name, value string, isDev bool) {
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

	key = strings.TrimSpace(line[:eq])
	key = strings.Trim(key, `"`)
	value = strings.TrimSpace(line[eq+1:])

	if key == "" {
		return "", "", false
	}

	return key, value, true
}

func unquoteCargoValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)

	return value
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
