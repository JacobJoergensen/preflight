package render

import (
	"strings"
	"time"

	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const statusFooterBoxWidth = 65

type footerStatus struct {
	Icon  string
	Color string
	Text  string
}

type footerMetadataLine struct {
	Label string
	Value string
}

func renderStatusFooter(ow *terminal.OutputWriter, status footerStatus, metadata []footerMetadataLine) {
	blueBar := terminal.Bold + terminal.Blue + "│" + terminal.Reset
	topBorder := terminal.Bold + terminal.Blue + "╭" + strings.Repeat("─", statusFooterBoxWidth) + "╮" + terminal.Reset
	botBorder := terminal.Bold + terminal.Blue + "╰" + strings.Repeat("─", statusFooterBoxWidth) + "╯" + terminal.Reset

	ow.PrintNewLines(2)
	ow.Println(topBorder)
	ow.Println(blueBar)
	ow.Printf("%s    %s%s%s  %s\n", blueBar, status.Color, status.Icon, terminal.Reset, status.Text)
	ow.Println(blueBar)

	if len(metadata) > 0 {
		labelWidth := footerLabelWidth(metadata)

		for _, line := range metadata {
			ow.Printf("%s       %s%s%s   %s\n",
				blueBar,
				terminal.Dim, padRight(line.Label, labelWidth), terminal.Reset,
				line.Value,
			)
		}

		ow.Println(blueBar)
	}

	ow.Println(botBorder)
}

func footerLabelWidth(lines []footerMetadataLine) int {
	var width int

	for _, line := range lines {
		if n := len(line.Label); n > width {
			width = n
		}
	}

	return width
}

func endedFooterLine(endedAt time.Time) footerMetadataLine {
	if endedAt.IsZero() {
		endedAt = time.Now()
	}

	return footerMetadataLine{
		Label: "Ended",
		Value: endedAt.Format("02-01-2006 15:04:05"),
	}
}
