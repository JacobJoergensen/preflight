package lockdiff

import (
	"bufio"
	"bytes"
	"strings"
)

type yarnParser struct{}

func (yarnParser) Ecosystem() string { return "node" }

func (yarnParser) Parse(data []byte) (map[string]string, error) {
	packages := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var headerNames []string
	versionAssigned := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")

		if !indented {
			headerNames = nil
			versionAssigned = false

			if !strings.HasSuffix(trimmed, ":") {
				continue
			}

			if strings.HasPrefix(trimmed, "__metadata") {
				continue
			}

			headerNames = parseYarnHeaderNames(strings.TrimSuffix(trimmed, ":"))

			continue
		}

		if versionAssigned || len(headerNames) == 0 {
			continue
		}

		version := parseYarnVersionLine(trimmed)

		if version == "" {
			continue
		}

		for _, name := range headerNames {
			packages[name] = version
		}

		versionAssigned = true
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

func parseYarnHeaderNames(header string) []string {
	var names []string

	seen := make(map[string]struct{})

	for spec := range strings.SplitSeq(header, ",") {
		spec = strings.TrimSpace(spec)
		spec = strings.Trim(spec, `"`)

		if spec == "" {
			continue
		}

		atIdx := strings.LastIndex(spec, "@")

		if atIdx <= 0 {
			continue
		}

		name := spec[:atIdx]

		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		names = append(names, name)
	}

	return names
}

func parseYarnVersionLine(trimmed string) string {
	const prefix = "version"

	version, ok := strings.CutPrefix(trimmed, prefix)

	if !ok {
		return ""
	}

	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, ":")
	version = strings.TrimSpace(version)
	version = strings.Trim(version, `"`)

	return version
}

func init() {
	Register("yarn.lock", yarnParser{})
}
