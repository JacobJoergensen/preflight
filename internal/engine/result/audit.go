package result

import (
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/model"
)

type AuditReport = Report[AuditItem]

type AuditItem struct {
	Project       string
	ScopeID       string
	ScopeDisplay  string
	Priority      int
	CommandLine   string
	ExitCode      int
	OK            bool
	SeverityRank  int
	Findings      []model.Finding
	Manifest      string
	Output        string
	ErrText       string
	Skipped       bool
	SkipReason    string
	StartedAt     time.Time
	EndedAt       time.Time
	ElapsedMillis int64
}

func FromAuditResult(scopeID, scopeDisplay string, priority int, ar ecosystem.AuditResult, startedAt, endedAt time.Time) AuditItem {
	item := AuditItem{
		ScopeID:       scopeID,
		ScopeDisplay:  scopeDisplay,
		Priority:      priority,
		CommandLine:   ar.CommandLine,
		ExitCode:      ar.ExitCode,
		OK:            ar.OK,
		SeverityRank:  ar.SeverityRank,
		Findings:      ar.Findings,
		Manifest:      ar.Manifest,
		Output:        ar.Output,
		Skipped:       ar.Skipped,
		SkipReason:    ar.SkipReason,
		StartedAt:     startedAt,
		EndedAt:       endedAt,
		ElapsedMillis: endedAt.Sub(startedAt).Milliseconds(),
	}

	if ar.Err != nil {
		item.ErrText = ar.Err.Error()
	}

	return item
}
