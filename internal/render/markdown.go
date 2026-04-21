package render

import "strings"

func escapeMarkdownCell(value string) string {
	escaped := strings.ReplaceAll(value, "|", "\\|")
	escaped = strings.ReplaceAll(escaped, "\n", " ")
	return escaped
}
