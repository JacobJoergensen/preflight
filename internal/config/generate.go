package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/manifest"
)

func Generate(workDir string, filesystem fs.FS) ([]byte, error) {
	loader := manifest.NewLoader(workDir)
	loader.FS = filesystem

	var scopes []string

	for _, packageType := range []string{manifest.PackageTypeComposer, manifest.PackageTypeJS, manifest.PackageTypeGo, manifest.PackageTypePython, manifest.PackageTypeRuby} {
		if _, ok := loader.DetectPackageManager(packageType); ok {
			scopes = append(scopes, packageType)
		}
	}

	withEnv := loader.FileExists(".env.example")

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

	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)

	if err := encoder.Encode(&config); err != nil {
		return nil, fmt.Errorf("encode preflight.yml: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

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

func buildProfile(scopes []string, withEnv bool) Profile {
	var profile Profile

	if len(scopes) > 0 {
		profile.Check = &Command{Scope: new(append([]string(nil), scopes...))}
		profile.Fix = &Command{Scope: new(append([]string(nil), scopes...))}
		profile.List = &Command{Scope: new(append([]string(nil), scopes...))}
	} else {
		profile.Check = &Command{}
		profile.Fix = &Command{}
		profile.List = &Command{}
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
