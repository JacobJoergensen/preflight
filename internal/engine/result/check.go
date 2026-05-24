package result

import (
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/model"
)

type CheckReport = Report[CheckItem]

type CheckItem struct {
	Project       string
	ScopeID       string
	ScopeDisplay  string
	Priority      int
	Messages      []model.Message
	Outdated      []ecosystem.OutdatedPackage
	StartedAt     time.Time
	EndedAt       time.Time
	ElapsedMillis int64

	// ProjectSignals are filesystem facts (paths that exist) for the scope, not check messages.
	ProjectSignals []string
	// FixPMHint is the package-manager id for `preflight fix --pm=…` when applicable (empty if unknown or N/A).
	FixPMHint string
}

func (c CheckItem) Errors() []model.Message {
	return c.bySeverity(model.SeverityError)
}

func (c CheckItem) Warnings() []model.Message {
	return c.bySeverity(model.SeverityWarning)
}

func (c CheckItem) Successes() []model.Message {
	return c.bySeverity(model.SeveritySuccess)
}

func (c CheckItem) bySeverity(severity model.Severity) []model.Message {
	var filtered []model.Message

	for _, message := range c.Messages {
		if message.Severity == severity {
			filtered = append(filtered, message)
		}
	}

	return filtered
}
