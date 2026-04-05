package manifest

import (
	"strings"
)

func (l Loader) ReadPythonVersionPin() string {
	return l.readToolVersionPinFile(".python-version")
}

func (l Loader) ReadRubyVersionPin() string {
	return l.readToolVersionPinFile(".ruby-version")
}

func (l Loader) ReadNodeVersionPinSource() (pin string, label string) {
	if l.FileExists(".nvmrc") {
		if version := l.readToolVersionPinFile(".nvmrc"); version != "" {
			return version, ".nvmrc"
		}
	}

	if l.FileExists(".node-version") {
		if version := l.readToolVersionPinFile(".node-version"); version != "" {
			return version, ".node-version"
		}
	}

	return "", ""
}

func (l Loader) readToolVersionPinFile(name string) string {
	if !l.FileExists(name) {
		return ""
	}

	raw, err := l.readFile(name)

	if err != nil {
		return ""
	}

	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])

		if line == "" {
			continue
		}

		return line
	}

	return ""
}
