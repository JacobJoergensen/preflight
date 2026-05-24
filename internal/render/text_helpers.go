package render

import "fmt"

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
}

func projectStatusLine(count, total int, verb string) string {
	return fmt.Sprintf("%d of %d project%s %s", count, total, pluralSuffix(total), verb)
}
