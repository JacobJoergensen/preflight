package result

import (
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/model"
)

type CheckReport struct {
	StartedAt time.Time
	EndedAt   time.Time
	Canceled  bool
	Items     []CheckItem
	Projects  []Project
}

type CheckItem struct {
	Project       string
	ScopeID       string
	ScopeDisplay  string
	Priority      int
	Messages      []model.Message
	Outdated      []adapter.OutdatedPackage
	StartedAt     time.Time
	EndedAt       time.Time
	ElapsedMillis int64

	// ProjectSignals are filesystem / manifest facts (paths that exist) for the scope, not adapter messages.
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
