package envfile

import (
	"bufio"
	"bytes"
	"strings"
	"unicode"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func ParseKeys(content []byte) []string {
	content = bytes.TrimPrefix(content, utf8BOM)
	scanner := bufio.NewScanner(bytes.NewReader(content))

	seen := make(map[string]struct{})
	var keys []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(strings.ToLower(line), "export ") {
			line = strings.TrimSpace(line[len("export "):])
		}

		keyPart, _, ok := strings.Cut(line, "=")

		if !ok {
			continue
		}

		key := strings.TrimSpace(keyPart)
		key = strings.Trim(key, `"'`)

		if key == "" || !isValidEnvKey(key) {
			continue
		}

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	return keys
}

func isValidEnvKey(key string) bool {
	for i, char := range key {
		if i == 0 {
			if char != '_' && !unicode.IsLetter(char) {
				return false
			}

			continue
		}

		if char != '_' && char != '.' && !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return false
		}
	}

	return len(key) > 0
}
