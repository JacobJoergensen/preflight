package lockdiff

import (
	"strings"

	"github.com/JacobJoergensen/preflight/internal/semver"
)

const goModSuffix = "/go.mod"

type goSumParser struct{}

func (goSumParser) Ecosystem() string { return "go" }

func (goSumParser) Parse(data []byte) (map[string]string, error) {
	modules := make(map[string]string)

	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		fields := strings.Fields(line)

		if len(fields) < 2 {
			continue
		}

		module := fields[0]
		version := strings.TrimSuffix(fields[1], goModSuffix)

		existing, ok := modules[module]

		if !ok || semver.Compare(version, existing) > 0 {
			modules[module] = version
		}
	}

	return modules, nil
}

func init() {
	Register("go.sum", goSumParser{})
}
