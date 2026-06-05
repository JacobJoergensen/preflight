package render

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func visibleWidth(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

func wrapVisible(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	var current strings.Builder
	currentWidth := 0

	for word := range strings.SplitSeq(text, " ") {
		wordWidth := visibleWidth(word)

		if currentWidth == 0 {
			current.WriteString(word)
			currentWidth = wordWidth
			continue
		}

		if currentWidth+1+wordWidth > width {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
			currentWidth = wordWidth
			continue
		}

		current.WriteByte(' ')
		current.WriteString(word)
		currentWidth += 1 + wordWidth
	}

	return append(lines, current.String())
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
}

func projectStatusLine(count, total int, verb string) string {
	return fmt.Sprintf("%d of %d project%s %s", count, total, pluralSuffix(total), verb)
}
