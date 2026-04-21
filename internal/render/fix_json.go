package render

import (
	"io"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type JSONFixRenderer struct {
	Out io.Writer
}

func (r JSONFixRenderer) Render(report result.FixReport) error {
	if terminal.Quiet {
		return encodeJSON(r.Out, quietFixPayload(report), false)
	}

	return encodeJSON(r.Out, report, true)
}

func quietFixPayload(report result.FixReport) any {
	type quietItem struct {
		Project     string `json:"project,omitempty"`
		ScopeID     string `json:"scopeId"`
		ManagerName string `json:"managerName"`
		Error       string `json:"error,omitempty"`
	}

	type quietReport struct {
		Canceled bool        `json:"canceled"`
		Items    []quietItem `json:"items"`
	}

	items := make([]quietItem, 0, len(report.Items))

	for _, item := range report.Items {
		if item.Success {
			continue
		}

		items = append(items, quietItem{
			Project:     item.Project,
			ScopeID:     item.ScopeID,
			ManagerName: item.ManagerName,
			Error:       item.Error,
		})
	}

	return quietReport{
		Canceled: report.Canceled,
		Items:    items,
	}
}
