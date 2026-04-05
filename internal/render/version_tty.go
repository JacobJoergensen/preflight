package render

import (
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
	ow.Println(terminal.Bold + terminal.Cyan + "PreFlight - Version Information" + terminal.Reset + terminal.Bold)
	ow.Printf("%s\n", terminal.Border)

	ow.Printf("Version:         %s\n", report.Version)

	if report.CheckFailed {
		ow.Println("Latest version:  Unable to check")
	} else if report.Version == "development" || report.HasUpdate {
		ow.Printf("Latest version:  %s\n", report.LatestVersion)
	}

	ow.Printf("Platform:        %s\n", report.Platform)
	ow.Printf("%s", terminal.Border)

	if report.CheckFailed && report.CheckErrorText != "" {
		ow.PrintNewLines(1)
		ow.Printf("%s%s %s%s\n", terminal.Yellow, terminal.WarningSign, report.CheckErrorText, terminal.Reset)
	}

	return nil
}
