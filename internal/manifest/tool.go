package manifest

import (
	"cmp"
	"slices"
	"strings"
)

type Tool struct {
	Name        string
	Command     string
	PackageType string
	ConfigFile  string
	LockFile    string
	VersionArgs []string
	InstallArgs []string
	ForceArgs   []string
}

const (
	PackageTypeJS       = "js"
	PackageTypeComposer = "composer"
	PackageTypeGo       = "go"
	PackageTypePython   = "python"
	PackageTypeRuby     = "ruby"
)

var Tools = map[string]Tool{
	"php": {
		Name:        "PHP",
		Command:     "php",
		VersionArgs: []string{"--version"},
	},
	"pie": {
		Name:        "PIE",
		Command:     "pie",
		VersionArgs: []string{"--version"},
	},
	"composer": {
		Name:        "Composer",
		Command:     "composer",
		PackageType: PackageTypeComposer,
		ConfigFile:  "composer.json",
		LockFile:    "composer.lock",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{"--no-cache"},
	},
	"npm": {
		Name:        "NPM",
		Command:     "npm",
		PackageType: PackageTypeJS,
		ConfigFile:  "package.json",
		LockFile:    "package-lock.json",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{"--force"},
	},
	"pnpm": {
		Name:        "PNPM",
		Command:     "pnpm",
		PackageType: PackageTypeJS,
		ConfigFile:  "package.json",
		LockFile:    "pnpm-lock.yaml",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{"--force"},
	},
	"yarn": {
		Name:        "Yarn",
		Command:     "yarn",
		PackageType: PackageTypeJS,
		ConfigFile:  "package.json",
		LockFile:    "yarn.lock",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{"--force"},
	},
	"bun": {
		Name:        "Bun",
		Command:     "bun",
		PackageType: PackageTypeJS,
		ConfigFile:  "package.json",
		LockFile:    "bun.lock",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{"--force"},
	},
	"go": {
		Name:        "Go Modules",
		Command:     "go",
		PackageType: PackageTypeGo,
		ConfigFile:  "go.mod",
		LockFile:    "go.sum",
		VersionArgs: []string{"version"},
		InstallArgs: []string{"mod", "tidy"},
		ForceArgs:   []string{"-mod=mod"},
	},
	"python": {
		Name:    "Python",
		Command: "python",
	},
	"python3": {
		Name:    "Python 3",
		Command: "python3",
	},
	"pip": {
		Name:        "pip",
		Command:     "pip",
		PackageType: PackageTypePython,
		ConfigFile:  "requirements.txt",
		LockFile:    "",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install", "-r", "requirements.txt"},
		ForceArgs:   []string{"--upgrade"},
	},
	"poetry": {
		Name:        "Poetry",
		Command:     "poetry",
		PackageType: PackageTypePython,
		ConfigFile:  "pyproject.toml",
		LockFile:    "poetry.lock",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{"--sync"},
	},
	"uv": {
		Name:        "uv",
		Command:     "uv",
		PackageType: PackageTypePython,
		ConfigFile:  "pyproject.toml",
		LockFile:    "uv.lock",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"sync"},
		ForceArgs:   []string{"--frozen"},
	},
	"pipenv": {
		Name:        "Pipenv",
		Command:     "pipenv",
		PackageType: PackageTypePython,
		ConfigFile:  "Pipfile",
		LockFile:    "Pipfile.lock",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{},
	},
	"pdm": {
		Name:        "PDM",
		Command:     "pdm",
		PackageType: PackageTypePython,
		ConfigFile:  "pyproject.toml",
		LockFile:    "pdm.lock",
		VersionArgs: []string{"--version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{},
	},
	"ruby": {
		Name:        "Ruby",
		Command:     "ruby",
		VersionArgs: []string{"-e", "print RUBY_VERSION"},
	},
	"bundle": {
		Name:        "Bundler",
		Command:     "bundle",
		PackageType: PackageTypeRuby,
		ConfigFile:  "Gemfile",
		LockFile:    "Gemfile.lock",
		VersionArgs: []string{"version"},
		InstallArgs: []string{"install"},
		ForceArgs:   []string{"--redownload"},
	},
	"node": {
		Name:        "Node.js",
		Command:     "node",
		VersionArgs: []string{"--version"},
	},
}

func GetTool(command string) (Tool, bool) {
	tool, exists := Tools[strings.ToLower(command)]
	return tool, exists
}

func GetToolsByPackageType(packageType string) []Tool {
	var tools []Tool

	for _, tool := range Tools {
		if tool.PackageType == packageType {
			tools = append(tools, tool)
		}
	}

	slices.SortFunc(tools, func(a, b Tool) int {
		return cmp.Compare(a.Command, b.Command)
	})

	return tools
}

func GetPackageType(command string) (string, bool) {
	tool, exists := Tools[strings.ToLower(command)]

	if !exists || tool.PackageType == "" {
		return "", false
	}

	return tool.PackageType, true
}

func AnyMatchesPackageType(commands []string, packageType string) bool {
	for _, cmd := range commands {
		if pt, ok := GetPackageType(cmd); ok && pt == packageType {
			return true
		}
	}

	return false
}

func IsPackageType(name string) bool {
	switch name {
	case PackageTypeJS, PackageTypeComposer, PackageTypeGo, PackageTypePython, PackageTypeRuby:
		return true
	default:
		return false
	}
}

func ResolvePackageType(name string) string {
	if packageType, ok := GetPackageType(name); ok {
		return packageType
	}

	return name
}
