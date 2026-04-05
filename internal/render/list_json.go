package render

import (
	"io"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type JSONListRenderer struct {
	Out io.Writer
}

func (r JSONListRenderer) Render(report result.DependencyReport) error {
	if terminal.Quiet {
		return encodeJSON(r.Out, quietListPayload(report), false)
	}

	return encodeJSON(r.Out, report, true)
}

func quietListPayload(report result.DependencyReport) any {
	type quietReport struct {
		Dependencies map[string][]string `json:"dependencies"`
	}

	return quietReport{Dependencies: report.Dependencies}
}
