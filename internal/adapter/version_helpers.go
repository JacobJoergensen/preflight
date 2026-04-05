package adapter

import (
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type versionFeedback struct {
	Feedback        string
	EOLWarning      string
	ShouldWarnEOL   bool
	ShouldError     bool
	ShouldSuccess   bool
	ShouldWarnExtra bool
}

func buildVersionFeedback(runtime string, display string, installedVersion string, required string, versionPrefix string) versionFeedback {
	isValid, _ := semver.ValidateVersion(installedVersion, required)
	feedback := fmt.Sprintf("Installed %s%s (%s ⟶ required %s)", terminal.Reset, display, installedVersion, required)

	if isEOL(runtime, versionPrefix) {
		return versionFeedback{
			Feedback:        feedback,
			EOLWarning:      fmt.Sprintf("Installed %s%s (%s ⟶ End-of-Life), consider upgrading!", terminal.Reset, display, installedVersion),
			ShouldWarnEOL:   true,
			ShouldWarnExtra: isValid,
		}
	}

	if !isValid {
		return versionFeedback{Feedback: feedback, ShouldError: true}
	}

	return versionFeedback{Feedback: feedback, ShouldSuccess: true}
}

// Uses minimum-version rules (go 1.26 allows 1.26.1), not npm-style exact semver.
func buildGoVersionFeedback(installedVersion, required, versionPrefix string) versionFeedback {
	ok := semver.MatchMinimumVersion(installedVersion, required)
	feedback := fmt.Sprintf("Installed %sGo (%s ⟶ required %s)", terminal.Reset, installedVersion, required)

	if isEOL("go", versionPrefix) {
		return versionFeedback{
			Feedback:        feedback,
			EOLWarning:      fmt.Sprintf("Installed %sGo (%s ⟶ End-of-Life), consider upgrading!", terminal.Reset, installedVersion),
			ShouldWarnEOL:   true,
			ShouldWarnExtra: ok,
		}
	}

	if !ok {
		return versionFeedback{Feedback: feedback, ShouldError: true}
	}

	return versionFeedback{Feedback: feedback, ShouldSuccess: true}
}

// Bare versions like "20" mean minimum 20.x, not exact equality to "20.0.0".
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

func buildNodeEngineFeedback(installedVersion, enginesField, versionPrefix string) versionFeedback {
	ok := nodeEngineSatisfiedByRuntime(installedVersion, enginesField)
	feedback := fmt.Sprintf("Installed %sNode.js (%s ⟶ required %s)", terminal.Reset, installedVersion, enginesField)

	if isEOL("node", versionPrefix) {
		return versionFeedback{
			Feedback:        feedback,
			EOLWarning:      fmt.Sprintf("Installed %sNode.js (%s ⟶ End-of-Life), consider upgrading!", terminal.Reset, installedVersion),
			ShouldWarnEOL:   true,
			ShouldWarnExtra: ok,
		}
	}

	if !ok {
		return versionFeedback{Feedback: feedback, ShouldError: true}
	}

	return versionFeedback{Feedback: feedback, ShouldSuccess: true}
}
