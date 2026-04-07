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
	type outdatedPackage struct {
		Name    string `json:"name"`
		Current string `json:"current"`
		Latest  string `json:"latest"`
	}

	type quietReport struct {
		Dependencies map[string][]string          `json:"dependencies"`
		Outdated     map[string][]outdatedPackage `json:"outdated,omitempty"`
	}

	qr := quietReport{Dependencies: report.Dependencies}

	if len(report.Outdated) > 0 {
		qr.Outdated = make(map[string][]outdatedPackage)

		for id, pkgs := range report.Outdated {
			outdatedPkgs := make([]outdatedPackage, len(pkgs))

			for i, pkg := range pkgs {
				outdatedPkgs[i] = outdatedPackage{
					Name:    pkg.Name,
					Current: pkg.Current,
					Latest:  pkg.Latest,
				}
			}

			qr.Outdated[id] = outdatedPkgs
		}
	}

	return qr
}
