package config

import (
	"bytes"
	"fmt"

	"github.com/goccy/go-yaml"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/fs"
)

const headerComment = `# PreFlight — https://github.com/JacobJoergensen/preflight
# CLI flags override values from this file. Profile resolution:
#   --profile > $PREFLIGHT_PROFILE > profile: below > "default"
#
# Named scripts for: preflight run <name>
#   profiles.<profile>.run.scripts.<name> sets exactly one of:
#     js, composer, go, ruby, python
# Example:
#   run:
#     scripts:
#       test:
#         js: test
#       phpunit:
#         composer: test
#

`

func Generate(workDir string, filesystem fs.FS) ([]byte, error) {
	rc := ecosystem.RunContext{WorkDir: workDir, FS: filesystem}

	specs, err := ecosystem.Select()
	if err != nil {
		return nil, fmt.Errorf("list ecosystems: %w", err)
	}

	var scopes []string

	for _, spec := range specs {
		if !spec.CanFix() {
			continue
		}

		if _, ok := spec.Resolve(rc); ok {
			scopes = append(scopes, spec.Name)
		}
	}

	withEnv := rc.FileExists(".env.example")

	config := File{
		Version: SchemaVersion,
		Profile: "default",
		Profiles: map[string]Profile{
			"default": buildProfile(scopes, withEnv),
			"ci":      buildCIProfile(scopes),
		},
	}

	var buffer bytes.Buffer
	buffer.WriteString(headerComment)

	encoder := yaml.NewEncoder(&buffer, yaml.Indent(2))
	defer func() { _ = encoder.Close() }()

	if err := encoder.Encode(&config); err != nil {
		return nil, fmt.Errorf("encode preflight.yml: %w", err)
	}

	return buffer.Bytes(), nil
}

func buildProfile(scopes []string, withEnv bool) Profile {
	var profile Profile

	if len(scopes) > 0 {
		profile.Check = &Command{Only: new(append([]string(nil), scopes...))}
		profile.Fix = &Command{Only: new(append([]string(nil), scopes...))}
	} else {
		profile.Check = &Command{}
		profile.Fix = &Command{}
	}

	if withEnv {
		profile.Check.WithEnv = new(true)
	}

	return profile
}

func buildCIProfile(scopes []string) Profile {
	profile := buildProfile(scopes, false)

	if profile.Check == nil {
		profile.Check = &Command{}
	}

	profile.Check.WithEnv = new(false)

	return profile
}
