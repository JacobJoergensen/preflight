package result

import "time"

// Report is the shared envelope every scoped command produces: the kept items
// plus run timing, cancellation, and per-project summaries for monorepos.
type Report[T any] struct {
	StartedAt time.Time
	EndedAt   time.Time
	Canceled  bool
	Items     []T
	Projects  []Project
}
