package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

// ProjectSignals returns short, factual lines about manifests and lockfiles for a check scope.
func ProjectSignals(scopeID string, loader manifest.Loader) []string {
	switch scopeID {
	case "js":
		return jsProjectSignals(loader)
	case "node":
		return nodeProjectSignals(loader)
	case "composer":
		return composerProjectSignals(loader)
	case "go":
		return goProjectSignals(loader)
	case "python":
		return pythonProjectSignals(loader)
	case "ruby":
		return rubyProjectSignals(loader)
	case "php":
		return phpProjectSignals()
	case "env":
		return envProjectSignals(loader)
	default:
		return nil
	}
}

// FixPMHint returns the `--pm` value for `preflight fix` when it maps cleanly to this scope.
func FixPMHint(scopeID string, loader manifest.Loader) string {
	switch scopeID {
	case "js":
		packageManager, ok := loader.DetectPackageManager(manifest.PackageTypeJS)

		if ok {
			return packageManager.Command()
		}
	case "composer":
		return "composer"
	case "go":
		return "go"
	case "python":
		packageManager, ok := loader.DetectPackageManager(manifest.PackageTypePython)

		if ok {
			return packageManager.Command()
		}
	case "ruby":
		return "bundle"
	case "node", "php":
		return ""
	default:
		return ""
	}

	return ""
}

func jsProjectSignals(l manifest.Loader) []string {
	lockOrder := []string{
		"package-lock.json",
		"pnpm-lock.yaml",
		"yarn.lock",
		"bun.lock",
		"bun.lockb",
	}

	lockToTool := map[string]string{
		"package-lock.json": "npm",
		"pnpm-lock.yaml":    "pnpm",
		"yarn.lock":         "yarn",
		"bun.lock":          "bun",
		"bun.lockb":         "bun",
	}

	var lockfiles []string

	ecosystems := make(map[string]struct{})

	for _, name := range lockOrder {
		if !l.FileExists(name) {
			continue
		}

		lockfiles = append(lockfiles, name)

		if tool, ok := lockToTool[name]; ok {
			ecosystems[tool] = struct{}{}
		}
	}

	var lines []string

	if l.FileExists("package.json") {
		lines = append(lines, "package.json exists")

		if raw, err := l.FS.ReadFile(filepath.Join(l.WorkDir, "package.json")); err == nil {
			lines = append(lines, jsExtraSignalsFromPackageJSON(raw)...)
		}
	}

	if l.FileExists("pnpm-workspace.yaml") {
		lines = append(lines, "pnpm-workspace.yaml exists")
	}

	if l.FileExists("bunfig.toml") {
		lines = append(lines, "bunfig.toml exists")
	}

	for _, lockfile := range lockfiles {
		lines = append(lines, lockfile+" exists")
	}

	if len(ecosystems) > 1 {
		lines = append(lines, "Note: lockfiles from more than one JS package manager — pick one tool and remove stray lockfiles when you can.")
	}

	return lines
}

func jsExtraSignalsFromPackageJSON(raw []byte) []string {
	var probe struct {
		PackageManager string          `json:"packageManager"`
		Workspaces     json.RawMessage `json:"workspaces"`
	}

	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}

	var output []string

	if packageManager := strings.TrimSpace(probe.PackageManager); packageManager != "" {
		output = append(output, "packageManager field: "+truncateSignalText(packageManager, 120))
	}

	if workspacesNonEmpty(probe.Workspaces) {
		output = append(output, "package.json workspaces configured")
	}

	return output
}

func workspacesNonEmpty(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}

	trimmed := strings.TrimSpace(string(raw))

	if trimmed == "" || trimmed == "null" {
		return false
	}

	var arr []any

	if json.Unmarshal(raw, &arr) == nil {
		return len(arr) > 0
	}

	var obj map[string]any

	if json.Unmarshal(raw, &obj) == nil {
		return len(obj) > 0
	}

	return true
}

func truncateSignalText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	if maxLen <= 1 {
		return "…"
	}

	return text[:maxLen-1] + "…"
}

func pythonShellEnvSignals() []string {
	var output []string

	if version := strings.TrimSpace(os.Getenv("VIRTUAL_ENV")); version != "" {
		output = append(output, "Shell: VIRTUAL_ENV="+shortSignalPath(version))
	}

	if version := strings.TrimSpace(os.Getenv("CONDA_PREFIX")); version != "" {
		output = append(output, "Shell: CONDA_PREFIX="+shortSignalPath(version))
	}

	return output
}

func rubyShellEnvSignals() []string {
	var output []string

	if version := strings.TrimSpace(os.Getenv("RBENV_VERSION")); version != "" {
		output = append(output, "Shell: RBENV_VERSION="+version)
	}

	if version := strings.TrimSpace(os.Getenv("GEM_HOME")); version != "" {
		output = append(output, "Shell: GEM_HOME="+shortSignalPath(version))
	}

	if version := strings.TrimSpace(os.Getenv("RUBY_ROOT")); version != "" {
		output = append(output, "Shell: RUBY_ROOT="+shortSignalPath(version))
	}

	return output
}

func shortSignalPath(path string) string {
	const pathTruncateMax = 96

	if len(path) <= pathTruncateMax {
		return path
	}

	return "…" + path[len(path)-(pathTruncateMax-1):]
}

func nodeProjectSignals(l manifest.Loader) []string {
	if !l.FileExists("package.json") {
		return nil
	}

	return []string{
		"package.json exists",
		"Node.js scope: checks engines.node vs `node` on PATH only.",
		"JavaScript scope: package manager, lockfiles, and npm dependencies.",
	}
}

func composerProjectSignals(l manifest.Loader) []string {
	var lines []string

	if l.FileExists("composer.json") {
		lines = append(lines, "composer.json exists")
	}

	if l.FileExists("composer.lock") {
		lines = append(lines, "composer.lock exists")
	}

	return lines
}

func goProjectSignals(l manifest.Loader) []string {
	var lines []string

	if l.FileExists("go.mod") {
		lines = append(lines, "go.mod exists")
	}

	if l.FileExists("go.sum") {
		lines = append(lines, "go.sum exists")
	}

	return lines
}

func pythonProjectSignals(l manifest.Loader) []string {
	packageManager, ok := l.DetectPackageManager(manifest.PackageTypePython)

	var lines []string

	if ok {
		if packageManager.ConfigFileExists && packageManager.Tool.ConfigFile != "" {
			lines = append(lines, packageManager.Tool.ConfigFile+" exists")
		}

		if packageManager.LockFileExists && packageManager.Tool.LockFile != "" {
			lines = append(lines, packageManager.Tool.LockFile+" exists")
		}
	}

	if pin := l.ReadPythonVersionPin(); pin != "" {
		lines = append(lines, ".python-version pins "+truncateSignalText(pin, 48))
	}

	lines = append(lines, pythonShellEnvSignals()...)

	if len(lines) == 0 {
		return nil
	}

	return lines
}

func rubyProjectSignals(l manifest.Loader) []string {
	packageManager, ok := l.DetectPackageManager(manifest.PackageTypeRuby)

	var lines []string

	if ok {
		if l.FileExists("Gemfile") {
			lines = append(lines, "Gemfile exists")
		} else if l.FileExists("gems.rb") {
			lines = append(lines, "gems.rb exists")
		}

		if packageManager.LockFileExists && packageManager.Tool.LockFile != "" && l.FileExists(packageManager.Tool.LockFile) {
			lines = append(lines, packageManager.Tool.LockFile+" exists")
		}
	}

	if pin := l.ReadRubyVersionPin(); pin != "" {
		lines = append(lines, ".ruby-version pins "+truncateSignalText(pin, 48))
	}

	lines = append(lines, rubyShellEnvSignals()...)

	if len(lines) == 0 {
		return nil
	}

	return lines
}

func phpProjectSignals() []string {
	// Composer manifests belong under the Composer scope only, PHP checks runtime/extensions.
	return nil
}

func envProjectSignals(l manifest.Loader) []string {
	var lines []string

	if l.FileExists(".env.example") {
		lines = append(lines, ".env.example exists")
	}

	if l.FileExists(".env") {
		lines = append(lines, ".env exists")
	}

	return lines
}
