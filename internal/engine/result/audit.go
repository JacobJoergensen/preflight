package result

import (
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
)

type AuditReport struct {
	StartedAt time.Time
	EndedAt   time.Time
	Canceled  bool
	Items     []AuditItem
}

type AuditItem struct {
	ScopeID       string
	ScopeDisplay  string
	Priority      int
	CommandLine   string
	ExitCode      int
	OK            bool
	SeverityRank  int
	Counts        map[string]int
	Output        string
	ErrText       string
	StartedAt     time.Time
	EndedAt       time.Time
	ElapsedMillis int64
}

func FromAdapterAudit(scopeID, scopeDisplay string, priority int, ar adapter.AuditResult, startedAt, endedAt time.Time) AuditItem {
	item := AuditItem{
		ScopeID:       scopeID,
		ScopeDisplay:  scopeDisplay,
		Priority:      priority,
		CommandLine:   ar.CommandLine,
		ExitCode:      ar.ExitCode,
		OK:            ar.OK,
		SeverityRank:  ar.SeverityRank,
		Counts:        ar.Counts,
		Output:        ar.Output,
		StartedAt:     startedAt,
		EndedAt:       endedAt,
		ElapsedMillis: endedAt.Sub(startedAt).Milliseconds(),
	}

	if ar.Err != nil {
		item.ErrText = ar.Err.Error()
	}

	return item
}
