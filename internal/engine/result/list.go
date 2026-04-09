package result

import (
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
)

type DependencyReport struct {
	StartedAt    time.Time
	EndedAt      time.Time
	AdapterIDs   []string
	Displays     map[string]string
	Dependencies map[string][]string
	Outdated     map[string][]adapter.OutdatedPackage
	Elapsed      map[string]int64
}
