package manifest

import "strings"

type PackageManager struct {
	Tool             Tool
	ConfigFileExists bool
	LockFileExists   bool
}

func (pm PackageManager) Name() string {
	return pm.Tool.Name
}

func (pm PackageManager) Command() string {
	return pm.Tool.Command
}

func (pm PackageManager) LockFile() string {
	return pm.Tool.LockFile
}

type pythonDetector struct {
	lockFile   string
	configFile string
	command    string
}

var pythonDetectors = []pythonDetector{
	{"poetry.lock", "pyproject.toml", "poetry"},
	{"uv.lock", "pyproject.toml", "uv"},
	{"Pipfile.lock", "Pipfile", "pipenv"},
	{"pdm.lock", "pyproject.toml", "pdm"},
}

func (l Loader) DetectPackageManager(packageType string) (PackageManager, bool) {
	tools := GetToolsByPackageType(packageType)

	if len(tools) == 0 {
		return PackageManager{}, false
	}

	switch packageType {
	case PackageTypeJS:
		return l.detectJSPackageManager(tools)
	case PackageTypePython:
		return l.detectPythonPackageManager()
	case PackageTypeRuby:
		return l.detectRubyPackageManager()
	default:
		tool := tools[0]
		return l.createPackageManager(tool, l.FileExists(tool.ConfigFile), l.FileExists(tool.LockFile)), true
	}
}

func (l Loader) createPackageManager(tool Tool, configExists, lockExists bool) PackageManager {
	return PackageManager{
		Tool:             tool,
		ConfigFileExists: configExists,
		LockFileExists:   lockExists,
	}
}

func (l Loader) toolPackageManager(command string, configExists bool) (PackageManager, bool) {
	tool, ok := GetTool(command)

	if !ok {
		return PackageManager{}, false
	}

	lockExists := tool.LockFile != "" && l.FileExists(tool.LockFile)
	return l.createPackageManager(tool, configExists, lockExists), true
}

func (l Loader) detectJSPackageManager(tools []Tool) (PackageManager, bool) {
	var npmTool Tool
	var npmConfigExists bool

	for _, tool := range tools {
		if tool.Command == "npm" {
			npmTool = tool
			npmConfigExists = l.FileExists(tool.ConfigFile)
			continue
		}

		if tool.Command == "bun" {
			if l.FileExists("bun.lock") || l.FileExists("bun.lockb") {
				return l.createPackageManager(tool, l.FileExists(tool.ConfigFile), true), true
			}

			continue
		}

		if l.FileExists(tool.LockFile) {
			return l.createPackageManager(tool, l.FileExists(tool.ConfigFile), true), true
		}
	}

	if npmTool.Command != "" {
		return l.createPackageManager(npmTool, npmConfigExists, l.FileExists(npmTool.LockFile)), true
	}

	return PackageManager{}, false
}

func (l Loader) detectPythonPackageManager() (PackageManager, bool) {
	for _, d := range pythonDetectors {
		if l.FileExists(d.lockFile) && l.FileExists(d.configFile) {
			return l.toolPackageManager(d.command, true)
		}
	}

	if l.FileExists("requirements.txt") {
		return l.toolPackageManager("pip", true)
	}

	if l.FileExists("pyproject.toml") {
		if cmd := l.inferPythonToolFromPyproject(); cmd != "" {
			return l.toolPackageManager(cmd, true)
		}
	}

	if l.FileExists("Pipfile") {
		return l.toolPackageManager("pipenv", true)
	}

	return PackageManager{}, false
}

func (l Loader) inferPythonToolFromPyproject() string {
	raw, err := l.readFile("pyproject.toml")

	if err != nil {
		return ""
	}

	s := string(raw)

	switch {
	case strings.Contains(s, "[tool.poetry]"):
		return "poetry"
	case strings.Contains(s, "[tool.pdm]"):
		return "pdm"
	case strings.Contains(s, "[tool.uv]"):
		return "uv"
	case strings.Contains(s, "[project]"):
		return "uv"
	}

	return ""
}

func (l Loader) detectRubyPackageManager() (PackageManager, bool) {
	configExists := l.FileExists("Gemfile") || l.FileExists("gems.rb")

	if !configExists {
		return PackageManager{}, false
	}

	return l.toolPackageManager("bundle", configExists)
}
