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
	Outdated  map[string][]adapter.OutdatedPackage
}

type CheckItem struct {
	ScopeID       string
	ScopeDisplay  string
	Priority      int
	Errors        []model.Message
	Warnings      []model.Message
	Successes     []model.Message
	StartedAt     time.Time
	EndedAt       time.Time
	ElapsedMillis int64

	// ProjectSignals are filesystem / manifest facts (paths that exist) for the scope, not adapter messages.
	ProjectSignals []string
	// FixPMHint is the package-manager id for `preflight fix --pm=…` when applicable (empty if unknown or N/A).
	FixPMHint string
}
