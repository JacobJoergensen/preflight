package result

import (
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
)

type DependencyReport struct {
	StartedAt time.Time
	EndedAt   time.Time
	Canceled  bool
	Items     []DependencyItem
	Projects  []DependencyProject
}

type DependencyProject struct {
	RelativePath string
	Name         string
}

type DependencyItem struct {
	Project       string
	AdapterID     string
	Display       string
	Dependencies  []string
	Outdated      []adapter.OutdatedPackage
	ElapsedMillis int64
}
