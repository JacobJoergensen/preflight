package adapter

import "strings"

func trimFirstLine(text string) string {
	text = strings.TrimSpace(text)

	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}

	return strings.TrimSpace(text)
}

func lookupInstalledLower(installed map[string]string, key string) (string, bool) {
	if version, ok := installed[strings.ToLower(key)]; ok {
		return version, true
	}

	return "", false
}
