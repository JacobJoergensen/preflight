package render

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/JacobJoergensen/preflight/internal/model"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

const (
	maxToolchainDisplayLines   = 4
	maxToolchainLineRunes      = 96
	maxDepRowsPerSection       = 35
	toolchainBulletExtraSpaces = 1
)

func splitToolchainDisplay(line string) (label, detail string, split bool) {
	line = strings.TrimSpace(line)

	if line == "" {
		return "", "", false
	}

	const arrow = " ⟶ "

	if strings.Contains(line, arrow) {
		left, right, ok := strings.Cut(line, arrow)

		if ok {
			left, right = strings.TrimSpace(left), strings.TrimSpace(right)

			if left != "" && right != "" {
				return left, right, true
			}
		}
	}

	if i := strings.Index(line, " ("); i > 0 {
		trimmed := strings.TrimSpace(line)

		if strings.HasSuffix(trimmed, ")") {
			left := strings.TrimSpace(line[:i])
			right := strings.TrimSpace(line[i:])

			if left != "" && right != "" {
				return left, right, true
			}
		}
	}

	return "", line, false
}

func truncateTTYRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}

	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}

	runes := []rune(s)

	if len(runes) > maxRunes {
		if maxRunes <= 1 {
			return "…"
		}

		return string(runes[:maxRunes-1]) + "…"
	}

	return s
}

func printToolchainLinesTTY(ow *terminal.OutputWriter, lines []string) {
	if len(lines) == 0 {
		return
	}

	shown := lines
	overflow := 0

	if len(shown) > maxToolchainDisplayLines {
		overflow = len(shown) - maxToolchainDisplayLines
		shown = shown[:maxToolchainDisplayLines]
	}

	for _, line := range shown {
		spaces := ttyProjectBodySpaces

		if strings.HasPrefix(strings.TrimSpace(line), "•") {
			spaces = ttyProjectBodySpaces + toolchainBulletExtraSpaces
		}

		indent := terminal.Gray + strings.Repeat(" ", spaces) + terminal.Reset

		label, detail, ok := splitToolchainDisplay(line)

		label = truncateTTYRunes(label, maxToolchainLineRunes)
		detail = truncateTTYRunes(detail, maxToolchainLineRunes)

		if ok && label != "" {
			ow.Println(indent + label + " " + detail)
		} else {
			ow.Println(indent + detail)
		}
	}

	if overflow > 0 {
		indent := terminal.Gray + strings.Repeat(" ", ttyProjectBodySpaces) + terminal.Reset
		ow.Println(indent + terminal.Dim + overflowNMoreLine(overflow, "toolchain line", "toolchain lines") + terminal.Reset)
	}
}

func overflowNMoreLine(count int, singular, plural string) string {
	if count == 1 {
		return "… 1 more " + singular + " not shown"
	}

	return fmt.Sprintf("… %d more %s not shown", count, plural)
}

func printMessagesUniformCapped(ow *terminal.OutputWriter, messages []model.Message, color, symbol string, overflowLabel string) {
	if len(messages) == 0 {
		return
	}

	if len(messages) <= maxDepRowsPerSection {
		printMessagesUniform(ow, messages, color, symbol)
		return
	}

	printMessagesUniform(ow, messages[:maxDepRowsPerSection], color, symbol)

	overflow := len(messages) - maxDepRowsPerSection
	ow.Printf("%s%s … %s%s\n", terminal.Dim, strings.Repeat(" ", ttyProjectBodySpaces), overflowMoreDepsLine(overflow, overflowLabel), terminal.Reset)
}

func overflowMoreDepsLine(count int, label string) string {
	if count == 1 {
		return "1 more " + label + " not shown"
	}

	return fmt.Sprintf("%d more %s not shown", count, label)
}
