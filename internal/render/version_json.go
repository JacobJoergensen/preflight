package render

import (
	"io"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type JSONVersionRenderer struct {
	Out io.Writer
}

func (r JSONVersionRenderer) Render(report result.VersionReport) error {
	if terminal.Quiet {
		return encodeJSON(r.Out, quietVersionPayload(report), false)
	}

	return encodeJSON(r.Out, report, true)
}

func quietVersionPayload(report result.VersionReport) any {
	type quietReport struct {
		Version string `json:"version"`
	}

	return quietReport{Version: report.Version}
}
