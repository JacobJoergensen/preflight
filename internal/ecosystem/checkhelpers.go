package ecosystem

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const execErrorDetailMax = 400

var eolVersions = map[string]map[string]bool{
	"php": {
		"5.6": true,
		"7.0": true, "7.1": true, "7.2": true, "7.3": true, "7.4": true,
		"8.0": true, "8.1": true,
	},
	"node": {
		"10": true, "11": true, "12": true, "13": true, "14": true, "15": true, "16": true,
		"17": true, "18": true, "19": true, "20": true, "21": true, "23": true, "25": true,
	},
	"go": {
		"1.12": true, "1.13": true, "1.14": true, "1.15": true,
		"1.16": true, "1.17": true, "1.18": true, "1.19": true,
		"1.20": true, "1.21": true, "1.22": true, "1.23": true,
		"1.24": true,
	},
}

func IsEOL(runtime, versionPrefix string) bool {
	versions, ok := eolVersions[strings.ToLower(runtime)]
	if !ok {
		return false
	}

	return versions[versionPrefix]
}

func VersionFeedback(runtime, display, installed, required, versionPrefix string, satisfied bool) []model.Message {
	feedback := fmt.Sprintf("Installed %s%s (%s ⟶ required %s)", terminal.Reset, display, installed, required)

	if IsEOL(runtime, versionPrefix) {
		messages := []model.Message{{
			Severity: model.SeverityWarning,
			Text:     fmt.Sprintf("Installed %s%s (%s ⟶ End-of-Life), consider upgrading!", terminal.Reset, display, installed),
		}}

		if satisfied {
			messages = append(messages, model.Message{Severity: model.SeverityWarning, Text: feedback})
		}

		return messages
	}

	if !satisfied {
		return []model.Message{{Severity: model.SeverityError, Text: feedback}}
	}

	return []model.Message{{Severity: model.SeveritySuccess, Text: feedback}}
}

func FirstLine(text string) string {
	text = strings.TrimSpace(text)

	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}

	return strings.TrimSpace(text)
}

func ToolVersion(ctx context.Context, rc RunContext, manager Manager) (string, error) {
	result, err := rc.Runner.Run(ctx, manager.Command, manager.VersionArgs...)
	if err != nil {
		return "", err
	}

	return result.Stdout, nil
}

func ReadVersionPin(rc RunContext, name string) string {
	if !rc.FileExists(name) {
		return ""
	}

	raw, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, name))
	if err != nil {
		return ""
	}

	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])

		if line == "" {
			continue
		}

		return line
	}

	return ""
}

func MissingLockfileWarning(rc RunContext, manager Manager, hasDependencies bool) []model.Message {
	if !hasDependencies || manager.LockFile == "" {
		return nil
	}

	if manager.ConfigFile != "" && !rc.FileExists(manager.ConfigFile) {
		return nil
	}

	if rc.FileExists(manager.LockFile) {
		return nil
	}

	command := strings.TrimSpace(manager.Command + " " + strings.Join(manager.InstallArgs, " "))

	return []model.Message{{
		Severity: model.SeverityWarning,
		Text:     fmt.Sprintf("%s missing, run `%s` for reproducible installs", manager.LockFile, command),
	}}
}

func FormatExecFailure(label string, err error) string {
	if err == nil {
		return label
	}

	detail := err.Error()

	if cmdErr, ok := errors.AsType[*exec.CommandError](err); ok && cmdErr.Stderr != "" {
		detail = cmdErr.Stderr
	}

	if len(detail) > execErrorDetailMax {
		detail = detail[:execErrorDetailMax-len("…")] + "…"
	}

	return label + ": " + detail
}
