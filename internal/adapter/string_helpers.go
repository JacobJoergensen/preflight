package adapter

import "strings"

func trimFirstLine(text string) string {
	text = strings.TrimSpace(text)

	if index := strings.IndexByte(text, '\n'); index >= 0 {
		text = text[:index]
	}

	return strings.TrimSpace(text)
}

func lookupInstalledLower(installed map[string]string, key string) (string, bool) {
	if version, ok := installed[strings.ToLower(key)]; ok {
		return version, true
	}

	return "", false
}
