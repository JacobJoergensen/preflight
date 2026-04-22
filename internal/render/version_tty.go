package render

import (
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type TTYVersionRenderer struct {
	Out *terminal.OutputWriter
}

func (r TTYVersionRenderer) Render(report result.VersionReport) error {
	ow := r.Out

	if ow == nil {
		ow = terminal.NewOutputWriter()
	}

	if terminal.Quiet {
		ow.Println(report.Version)
		return nil
	}

	ow.PrintNewLines(1)

	ow.Printf("  %sVersion%s         %s\n", terminal.Dim, terminal.Reset, report.Version)

	if report.HasUpdate {
		ow.Printf("  %sLatest version%s  %s\n", terminal.Dim, terminal.Reset, report.LatestVersion)
	}

	if report.Commit != "" {
		ow.Printf("  %sCommit%s          %s\n", terminal.Dim, terminal.Reset, report.Commit)
	}

	if report.BuildDate != "" {
		ow.Printf("  %sBuilt%s           %s\n", terminal.Dim, terminal.Reset, formatBuildDate(report.BuildDate))
	}

	ow.Printf("  %sPlatform%s        %s\n", terminal.Dim, terminal.Reset, report.Platform)

	icon, color, text := versionStatusFromReport(report)
	renderStatusFooter(ow, footerStatus{Icon: icon, Color: color, Text: text}, versionFooterMetadata(report))

	return nil
}

func versionStatusFromReport(report result.VersionReport) (icon string, color string, text string) {
	if report.CheckErrorText != "" {
		return terminal.WarningSign, terminal.Yellow, "Update check failed: " + report.CheckErrorText
	}

	if report.HasUpdate {
		return terminal.Lightning, terminal.Yellow, "Update available: " + report.LatestVersion
	}

	return terminal.CheckMark, terminal.Green, "Up to date"
}

func versionFooterMetadata(report result.VersionReport) []footerMetadataLine {
	lines := []footerMetadataLine{endedFooterLine(report.EndedAt)}

	if report.ReleaseURL != "" {
		lines = append(lines, footerMetadataLine{Label: report.ReleaseURL})
	}

	return lines
}

func formatBuildDate(raw string) string {
	parsed, err := time.Parse(time.RFC3339, raw)

	if err != nil {
		return raw
	}

	return parsed.UTC().Format("2006-01-02 15:04 UTC")
}
