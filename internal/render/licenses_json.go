package render

import (
	"io"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const LicenseJSONSchemaVersion = 1

type JSONLicenseRenderer struct {
	Out io.Writer
}

type licenseViolationJSON struct {
	Package string `json:"package"`
	Version string `json:"version,omitempty"`
	License string `json:"license,omitempty"`
	Reason  string `json:"reason"`
}

type licenseItemJSON struct {
	ScopeID       string                 `json:"scopeId"`
	ScopeDisplay  string                 `json:"scopeDisplay"`
	Project       string                 `json:"project,omitempty"`
	Priority      int                    `json:"priority"`
	Inspected     int                    `json:"inspected"`
	Violations    []licenseViolationJSON `json:"violations,omitempty"`
	Err           string                 `json:"error,omitempty"`
	ElapsedMillis int64                  `json:"elapsedMillis,omitempty"`
	StartedAt     *time.Time             `json:"startedAt,omitempty"`
	EndedAt       *time.Time             `json:"endedAt,omitempty"`
}

func (r JSONLicenseRenderer) Render(report result.LicenseReport) error {
	if terminal.Quiet {
		return encodeJSON(r.Out, quietLicensePayload(report), false)
	}

	items := make([]licenseItemJSON, 0, len(report.Items))

	for _, item := range report.Items {
		items = append(items, licenseItemToJSON(item))
	}

	payload := reportJSON[licenseItemJSON]{
		SchemaVersion: LicenseJSONSchemaVersion,
		StartedAt:     report.StartedAt,
		EndedAt:       report.EndedAt,
		Canceled:      report.Canceled,
		Items:         items,
		Projects:      projectsToJSON(report.Projects),
	}

	return encodeJSON(r.Out, payload, true)
}

func licenseItemToJSON(item result.LicenseItem) licenseItemJSON {
	jsonItem := licenseItemJSON{
		ScopeID:       item.ScopeID,
		ScopeDisplay:  item.ScopeDisplay,
		Project:       item.Project,
		Priority:      item.Priority,
		Inspected:     item.Inspected,
		Violations:    licenseViolationsToJSON(item.Violations),
		Err:           item.ErrText,
		ElapsedMillis: item.ElapsedMillis,
	}

	if !item.StartedAt.IsZero() {
		jsonItem.StartedAt = new(item.StartedAt)
	}

	if !item.EndedAt.IsZero() {
		jsonItem.EndedAt = new(item.EndedAt)
	}

	return jsonItem
}

func licenseViolationsToJSON(violations []result.LicenseViolation) []licenseViolationJSON {
	if len(violations) == 0 {
		return nil
	}

	out := make([]licenseViolationJSON, 0, len(violations))

	for _, violation := range violations {
		out = append(out, licenseViolationJSON{
			Package: violation.Package,
			Version: violation.Version,
			License: violation.License,
			Reason:  violation.Reason,
		})
	}

	return out
}

func quietLicensePayload(report result.LicenseReport) any {
	type quietItem struct {
		ScopeID    string `json:"scopeId"`
		Violations int    `json:"violations"`
	}

	items := make([]quietItem, 0, len(report.Items))

	for _, item := range report.Items {
		items = append(items, quietItem{ScopeID: item.ScopeID, Violations: len(item.Violations)})
	}

	return struct {
		SchemaVersion int         `json:"schemaVersion"`
		Items         []quietItem `json:"items"`
	}{
		SchemaVersion: LicenseJSONSchemaVersion,
		Items:         items,
	}
}
