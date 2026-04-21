package render

import (
	"io"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

// ListJSONSchemaVersion is bumped when preflight list --json output shape changes incompatibly.
const ListJSONSchemaVersion = 1

type JSONListRenderer struct {
	Out io.Writer
}

type listReportJSON struct {
	SchemaVersion int               `json:"schemaVersion"`
	StartedAt     time.Time         `json:"startedAt"`
	EndedAt       time.Time         `json:"endedAt"`
	Items         []listItemJSON    `json:"items"`
	Projects      []listProjectJSON `json:"projects,omitempty"`
}

type listProjectJSON struct {
	RelativePath string `json:"relativePath"`
	Name         string `json:"name,omitempty"`
}

type listItemJSON struct {
	Project       string                `json:"project,omitempty"`
	AdapterID     string                `json:"adapterId"`
	Display       string                `json:"display"`
	Dependencies  []string              `json:"dependencies"`
	Outdated      []outdatedPackageJSON `json:"outdated,omitempty"`
	ElapsedMillis int64                 `json:"elapsedMillis,omitempty"`
}

func (r JSONListRenderer) Render(report result.DependencyReport) error {
	if terminal.Quiet {
		return encodeJSON(r.Out, quietListPayload(report), false)
	}

	items := make([]listItemJSON, 0, len(report.Items))

	for _, item := range report.Items {
		items = append(items, listItemJSON{
			Project:       item.Project,
			AdapterID:     item.AdapterID,
			Display:       item.Display,
			Dependencies:  item.Dependencies,
			Outdated:      outdatedPackagesToJSON(item.Outdated),
			ElapsedMillis: item.ElapsedMillis,
		})
	}

	payload := listReportJSON{
		SchemaVersion: ListJSONSchemaVersion,
		StartedAt:     report.StartedAt,
		EndedAt:       report.EndedAt,
		Items:         items,
		Projects:      listProjectsToJSON(report.Projects),
	}

	return encodeJSON(r.Out, payload, true)
}

func listProjectsToJSON(projects []result.DependencyProject) []listProjectJSON {
	if len(projects) == 0 {
		return nil
	}

	jsonProjects := make([]listProjectJSON, len(projects))

	for i, project := range projects {
		jsonProjects[i] = listProjectJSON{
			RelativePath: project.RelativePath,
			Name:         project.Name,
		}
	}

	return jsonProjects
}

func quietListPayload(report result.DependencyReport) any {
	type quietItem struct {
		Project      string                `json:"project,omitempty"`
		AdapterID    string                `json:"adapterId"`
		Dependencies []string              `json:"dependencies"`
		Outdated     []outdatedPackageJSON `json:"outdated,omitempty"`
	}

	type quietReport struct {
		Items []quietItem `json:"items"`
	}

	items := make([]quietItem, 0, len(report.Items))

	for _, item := range report.Items {
		items = append(items, quietItem{
			Project:      item.Project,
			AdapterID:    item.AdapterID,
			Dependencies: item.Dependencies,
			Outdated:     outdatedPackagesToJSON(item.Outdated),
		})
	}

	return quietReport{Items: items}
}
