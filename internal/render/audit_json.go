package render

import (
	"io"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

// AuditJSONSchemaVersion is bumped when preflight audit --json output shape changes incompatibly.
const AuditJSONSchemaVersion = 1

type JSONAuditRenderer struct {
	Out io.Writer
}

type auditReportJSON struct {
	Canceled      bool            `json:"canceled"`
	EndedAt       time.Time       `json:"endedAt"`
	Items         []auditItemJSON `json:"items"`
	SchemaVersion int             `json:"schemaVersion"`
	StartedAt     time.Time       `json:"startedAt"`
}

type auditItemJSON struct {
	CommandLine   string         `json:"commandLine,omitempty"`
	Counts        map[string]int `json:"counts,omitempty"`
	ElapsedMillis int64          `json:"elapsedMillis,omitempty"`
	EndedAt       *time.Time     `json:"endedAt,omitempty"`
	Err           string         `json:"error,omitempty"`
	ExitCode      int            `json:"exitCode"`
	OK            bool           `json:"ok"`
	Output        string         `json:"output,omitempty"`
	Priority      int            `json:"priority"`
	ScopeDisplay  string         `json:"scopeDisplay"`
	ScopeID       string         `json:"scopeId"`
	SeverityRank  int            `json:"severityRank"`
	SkipReason    string         `json:"skipReason,omitempty"`
	Skipped       bool           `json:"skipped"`
	StartedAt     *time.Time     `json:"startedAt,omitempty"`
}

func (r JSONAuditRenderer) Render(report result.AuditReport) error {
	if terminal.Quiet {
		return encodeJSON(r.Out, quietAuditPayload(report), false)
	}

	items := make([]auditItemJSON, 0, len(report.Items))

	for _, item := range report.Items {
		items = append(items, auditItemToJSON(item))
	}

	payload := auditReportJSON{
		Canceled:      report.Canceled,
		EndedAt:       report.EndedAt,
		Items:         items,
		SchemaVersion: AuditJSONSchemaVersion,
		StartedAt:     report.StartedAt,
	}

	return encodeJSON(r.Out, payload, true)
}

func auditItemToJSON(item result.AuditItem) auditItemJSON {
	jsonItem := auditItemJSON{
		CommandLine:   item.CommandLine,
		Counts:        item.Counts,
		ElapsedMillis: item.ElapsedMillis,
		Err:           item.ErrText,
		ExitCode:      item.ExitCode,
		OK:            item.OK,
		Output:        item.Output,
		Priority:      item.Priority,
		ScopeDisplay:  item.ScopeDisplay,
		ScopeID:       item.ScopeID,
		SeverityRank:  item.SeverityRank,
		SkipReason:    item.SkipReason,
		Skipped:       item.Skipped,
	}

	if !item.StartedAt.IsZero() {
		jsonItem.StartedAt = new(item.StartedAt)
	}

	if !item.EndedAt.IsZero() {
		jsonItem.EndedAt = new(item.EndedAt)
	}

	return jsonItem
}

func quietAuditPayload(report result.AuditReport) any {
	type quietItem struct {
		ScopeID      string `json:"scopeId"`
		OK           bool   `json:"ok"`
		Skipped      bool   `json:"skipped"`
		SeverityRank int    `json:"severityRank"`
	}

	items := make([]quietItem, 0, len(report.Items))

	for _, item := range report.Items {
		items = append(items, quietItem{
			ScopeID:      item.ScopeID,
			OK:           item.OK,
			Skipped:      item.Skipped,
			SeverityRank: item.SeverityRank,
		})
	}

	return struct {
		SchemaVersion int         `json:"schemaVersion"`
		Items         []quietItem `json:"items"`
	}{
		SchemaVersion: AuditJSONSchemaVersion,
		Items:         items,
	}
}
