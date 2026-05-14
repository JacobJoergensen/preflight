package lockdiff

import "strings"

type cargoLockParser struct{}

func (cargoLockParser) Ecosystem() string { return "rust" }

func (cargoLockParser) Parse(data []byte) (map[string]string, error) {
	packages := make(map[string]string)

	var name, version string
	insidePackage := false

	flush := func() {
		if name != "" && version != "" {
			packages[name] = version
		}

		name, version = "", ""
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == "[[package]]" {
			flush()
			insidePackage = true
			continue
		}

		if strings.HasPrefix(trimmed, "[") {
			flush()
			insidePackage = false
			continue
		}

		if !insidePackage {
			continue
		}

		if value, ok := strings.CutPrefix(trimmed, "name = "); ok {
			name = strings.Trim(value, `"`)
			continue
		}

		if value, ok := strings.CutPrefix(trimmed, "version = "); ok {
			version = strings.Trim(value, `"`)
		}
	}

	flush()

	return packages, nil
}

func init() {
	Register("Cargo.lock", cargoLockParser{})
}
