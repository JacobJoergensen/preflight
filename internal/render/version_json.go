package render

import (
	"io"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const VersionJSONSchemaVersion = 1

type JSONVersionRenderer struct {
	Out io.Writer
}

type versionReportJSON struct {
	SchemaVersion  int       `json:"schemaVersion"`
	StartedAt      time.Time `json:"startedAt"`
	EndedAt        time.Time `json:"endedAt"`
	Version        string    `json:"version"`
	Commit         string    `json:"commit,omitempty"`
	BuildDate      string    `json:"buildDate,omitempty"`
	Platform       string    `json:"platform"`
	LatestVersion  string    `json:"latestVersion,omitempty"`
	ReleaseURL     string    `json:"releaseUrl,omitempty"`
	HasUpdate      bool      `json:"hasUpdate"`
	CheckErrorText string    `json:"checkErrorText,omitempty"`
}

func (r JSONVersionRenderer) Render(report result.VersionReport) error {
	if terminal.Quiet {
		return encodeJSON(r.Out, quietVersionPayload(report), false)
	}

	payload := versionReportJSON{
		SchemaVersion:  VersionJSONSchemaVersion,
		StartedAt:      report.StartedAt,
		EndedAt:        report.EndedAt,
		Version:        report.Version,
		Commit:         report.Commit,
		BuildDate:      report.BuildDate,
		Platform:       report.Platform,
		LatestVersion:  report.LatestVersion,
		ReleaseURL:     report.ReleaseURL,
		HasUpdate:      report.HasUpdate,
		CheckErrorText: report.CheckErrorText,
	}

	return encodeJSON(r.Out, payload, true)
}

func quietVersionPayload(report result.VersionReport) any {
	type quietReport struct {
		Version string `json:"version"`
	}

	return quietReport{Version: report.Version}
}
