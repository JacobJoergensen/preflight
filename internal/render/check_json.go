package render

import (
	"io"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

// CheckJSONSchemaVersion is bumped when preflight check --json output shape changes incompatibly.
const CheckJSONSchemaVersion = 8

type JSONCheckRenderer struct {
	Out io.Writer
}

type outdatedPackageJSON struct {
	Name    string `json:"name"`
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

type checkReportJSON struct {
	SchemaVersion int                              `json:"schemaVersion"`
	StartedAt     time.Time                        `json:"startedAt"`
	EndedAt       time.Time                        `json:"endedAt"`
	Canceled      bool                             `json:"canceled"`
	Items         []checkItemJSON                  `json:"items"`
	Outdated      map[string][]outdatedPackageJSON `json:"outdated,omitempty"`
}

type checkItemJSON struct {
	ScopeID        string          `json:"scopeId"`
	ScopeDisplay   string          `json:"scopeDisplay"`
	Priority       int             `json:"priority"`
	Errors         []model.Message `json:"errors,omitempty"`
	Warnings       []model.Message `json:"warnings,omitempty"`
	Successes      []model.Message `json:"successes,omitempty"`
	StartedAt      *time.Time      `json:"startedAt,omitempty"`
	EndedAt        *time.Time      `json:"endedAt,omitempty"`
	ElapsedMillis  int64           `json:"elapsedMillis,omitempty"`
	ProjectSignals []string        `json:"projectSignals,omitempty"`
	FixPMHint      string          `json:"fixPmHint,omitempty"`
	Health         HealthCard      `json:"health"`
}

func (r JSONCheckRenderer) Render(report result.CheckReport) error {
	if terminal.Quiet {
		return encodeJSON(r.Out, quietCheckPayload(report), false)
	}

	items := make([]checkItemJSON, 0, len(report.Items))

	for _, item := range report.Items {
		items = append(items, checkItemToJSON(item))
	}

	payload := checkReportJSON{
		SchemaVersion: CheckJSONSchemaVersion,
		StartedAt:     report.StartedAt,
		EndedAt:       report.EndedAt,
		Canceled:      report.Canceled,
		Items:         items,
	}

	if len(report.Outdated) > 0 {
		payload.Outdated = make(map[string][]outdatedPackageJSON)

		for scopeID, pkgs := range report.Outdated {
			outdatedPkgs := make([]outdatedPackageJSON, len(pkgs))

			for i, pkg := range pkgs {
				outdatedPkgs[i] = outdatedPackageJSON{
					Name:    pkg.Name,
					Current: pkg.Current,
					Latest:  pkg.Latest,
				}
			}

			payload.Outdated[scopeID] = outdatedPkgs
		}
	}

	return encodeJSON(r.Out, payload, true)
}

func checkItemToJSON(item result.CheckItem) checkItemJSON {
	jsonItem := checkItemJSON{
		ScopeID:        item.ScopeID,
		ScopeDisplay:   item.ScopeDisplay,
		Priority:       item.Priority,
		Errors:         cloneMessages(item.Errors),
		Warnings:       cloneMessages(item.Warnings),
		Successes:      cloneMessages(item.Successes),
		ElapsedMillis:  item.ElapsedMillis,
		ProjectSignals: append([]string(nil), item.ProjectSignals...),
		FixPMHint:      item.FixPMHint,
		Health:         BuildHealthCard(item),
	}

	if !item.StartedAt.IsZero() {
		jsonItem.StartedAt = new(item.StartedAt)
	}

	if !item.EndedAt.IsZero() {
		jsonItem.EndedAt = new(item.EndedAt)
	}

	return jsonItem
}

func cloneMessages(src []model.Message) []model.Message {
	if len(src) == 0 {
		return nil
	}

	return append([]model.Message(nil), src...)
}

func quietCheckPayload(report result.CheckReport) any {
	type quietItem struct {
		ScopeID         string          `json:"scopeId"`
		ScopeDisplay    string          `json:"scopeDisplay"`
		Priority        int             `json:"priority"`
		Status          HealthStatus    `json:"status"`
		Summary         string          `json:"summary,omitempty"`
		ProjectSignals  []string        `json:"projectSignals,omitempty"`
		PrimaryNextStep string          `json:"primaryNextStep,omitempty"`
		Errors          []model.Message `json:"errors,omitempty"`
		Warnings        []model.Message `json:"warnings,omitempty"`
	}

	type quietReport struct {
		SchemaVersion int         `json:"schemaVersion"`
		StartedAt     time.Time   `json:"startedAt"`
		EndedAt       time.Time   `json:"endedAt"`
		Canceled      bool        `json:"canceled"`
		Items         []quietItem `json:"items"`
	}

	items := make([]quietItem, 0, len(report.Items))

	for _, item := range report.Items {
		if len(item.Errors) == 0 && len(item.Warnings) == 0 {
			continue
		}

		card := BuildHealthCard(item)

		items = append(items, quietItem{
			ScopeID:         item.ScopeID,
			ScopeDisplay:    item.ScopeDisplay,
			Priority:        item.Priority,
			Status:          card.Status,
			Summary:         card.Summary,
			ProjectSignals:  append([]string(nil), item.ProjectSignals...),
			PrimaryNextStep: card.PrimaryNextStep,
			Errors:          cloneMessages(item.Errors),
			Warnings:        cloneMessages(item.Warnings),
		})
	}

	return quietReport{
		SchemaVersion: CheckJSONSchemaVersion,
		StartedAt:     report.StartedAt,
		EndedAt:       report.EndedAt,
		Canceled:      report.Canceled,
		Items:         items,
	}
}
