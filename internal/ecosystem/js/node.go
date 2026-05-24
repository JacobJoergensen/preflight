package js

import (
	"context"
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/semver"
)

func NodeSpec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:            "node",
		DisplayName:     "Node.js",
		Priority:        3,
		RuntimeCommands: []string{"node"},
		Detect:          []ecosystem.Marker{{File: "package.json"}},
		Check:           checkNode,
	}
}

func checkNode(ctx context.Context, rc ecosystem.RunContext, _ ecosystem.Detection) []model.Message {
	if ctx.Err() != nil {
		return nil
	}

	config, err := loadConfig(rc)
	if err != nil {
		return []model.Message{{Severity: model.SeverityWarning, Text: err.Error()}}
	}

	if config.NodeVersion == "" {
		return nil
	}

	var messages []model.Message

	if pin, label := readNodeVersionPinSource(rc); pin != "" && label != "" && !nodeEngineSatisfiedByRuntime(pin, config.NodeVersion) {
		messages = append(messages, model.Message{Severity: model.SeverityWarning, Text: fmt.Sprintf(
			"%s pins %s but engines.node is %s — align these for CI vs local.",
			label, strings.TrimSpace(pin), strings.TrimSpace(config.NodeVersion),
		)})
	}

	nodeVersion, err := readNodeVersion(ctx, rc)
	if err != nil {
		messages = append(messages, model.Message{Severity: model.SeverityError, Text: fmt.Sprintf("Node is not installed or not on PATH: %v", err)})
		return messages
	}

	versionPrefix := strings.TrimPrefix(strings.Split(nodeVersion, ".")[0], "v")
	satisfied := nodeEngineSatisfiedByRuntime(nodeVersion, config.NodeVersion)
	messages = append(messages, ecosystem.VersionFeedback("node", "Node.js", nodeVersion, config.NodeVersion, versionPrefix, satisfied)...)

	return messages
}

func readNodeVersion(ctx context.Context, rc ecosystem.RunContext) (string, error) {
	result, err := rc.Runner.Run(ctx, "node", "--version")
	if err != nil {
		return "", err
	}

	return result.Stdout, nil
}

func nodeEngineSatisfiedByRuntime(installed, engines string) bool {
	installed = strings.TrimPrefix(strings.TrimSpace(installed), "v")
	engines = strings.TrimSpace(engines)

	if engines == "" {
		return true
	}

	if shouldUseNodeEnginesSemverRange(engines) {
		return semver.MatchVersionConstraint(installed, engines)
	}

	return semver.MatchMinimumVersion(installed, engines)
}

func shouldUseNodeEnginesSemverRange(engines string) bool {
	if strings.Contains(engines, "||") || strings.Contains(engines, " - ") {
		return true
	}

	if strings.ContainsAny(engines, "^~><=*") {
		return true
	}

	if strings.Contains(strings.ToLower(engines), "x") {
		return true
	}

	return false
}

func readNodeVersionPinSource(rc ecosystem.RunContext) (pin, label string) {
	if version := ecosystem.ReadVersionPin(rc, ".nvmrc"); version != "" {
		return version, ".nvmrc"
	}

	if version := ecosystem.ReadVersionPin(rc, ".node-version"); version != "" {
		return version, ".node-version"
	}

	return "", ""
}
