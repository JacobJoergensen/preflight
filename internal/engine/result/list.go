package result

import "time"

type DependencyReport struct {
	StartedAt    time.Time
	EndedAt      time.Time
	AdapterIDs   []string
	Dependencies map[string][]string
}
