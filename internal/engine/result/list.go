package result

import (
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
)

type DependencyReport struct {
	StartedAt    time.Time
	EndedAt      time.Time
	AdapterIDs   []string
	Dependencies map[string][]string
	Outdated     map[string][]adapter.OutdatedPackage
}
