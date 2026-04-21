package lockdiff

import (
	"regexp"
	"strings"
)

var gemSpecLine = regexp.MustCompile(`^    ([A-Za-z0-9_.\-]+) \(([^)]+)\)$`)

type gemfileParser struct{}

func (gemfileParser) Ecosystem() string { return "ruby" }

func (gemfileParser) Parse(data []byte) (map[string]string, error) {
	gems := make(map[string]string)

	var inGemSpecs bool

	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, " ") && strings.TrimSpace(line) != "" {
			inGemSpecs = line == "GEM"
			continue
		}

		if !inGemSpecs {
			continue
		}

		match := gemSpecLine.FindStringSubmatch(line)

		if len(match) != 3 {
			continue
		}

		gems[match[1]] = match[2]
	}

	return gems, nil
}

func init() {
	Register("Gemfile.lock", gemfileParser{})
}
