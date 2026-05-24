package config

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

const (
	FileName      = "preflight.yml"
	SchemaVersion = 1
)

type File struct {
	Version  int                `yaml:"version"`
	Profile  string             `yaml:"profile,omitempty"`
	Profiles map[string]Profile `yaml:"profiles,omitempty"`
}

type Profile struct {
	Check    *Command         `yaml:"check,omitempty"`
	Fix      *Command         `yaml:"fix,omitempty"`
	Audit    *AuditCommand    `yaml:"audit,omitempty"`
	Licenses *LicensesCommand `yaml:"licenses,omitempty"`
	Run      *RunBlock        `yaml:"run,omitempty"`
}

type LicensesCommand struct {
	Only  *[]string `yaml:"only,omitempty"`
	Allow *[]string `yaml:"allow,omitempty"`
	Deny  *[]string `yaml:"deny,omitempty"`
}

type AuditCommand struct {
	Only        *[]string `yaml:"only,omitempty"`
	MinSeverity *string   `yaml:"minSeverity,omitempty"`
	IgnoredCVEs *[]string `yaml:"ignoredCves,omitempty"`
}

type RunBlock struct {
	Scripts map[string]ScriptTargets `yaml:"scripts,omitempty"`
}

type Command struct {
	Only    *[]string `yaml:"only,omitempty"`
	WithEnv *bool     `yaml:"withEnv,omitempty"`
}

type ScriptTarget struct {
	JS       string `yaml:"js,omitempty"`
	Composer string `yaml:"composer,omitempty"`
	Go       string `yaml:"go,omitempty"`
	Ruby     string `yaml:"ruby,omitempty"`
	Python   string `yaml:"python,omitempty"`
	Rust     string `yaml:"rust,omitempty"`
}

type ScriptTargets []ScriptTarget

func (s *ScriptTargets) UnmarshalYAML(node ast.Node) error {
	switch node.Type() {
	case ast.MappingType:
		var single ScriptTarget

		if err := yaml.NodeToValue(node, &single); err != nil {
			return err
		}

		*s = []ScriptTarget{single}
	case ast.SequenceType:
		var multi []ScriptTarget

		if err := yaml.NodeToValue(node, &multi); err != nil {
			return err
		}

		*s = multi
	default:
		return fmt.Errorf("expected map or sequence, got %s", node.Type())
	}

	return nil
}

func (f File) validate() error {
	if f.Version != 0 && f.Version != SchemaVersion {
		return fmt.Errorf("preflight.yml: unsupported version %d (supported: %d)", f.Version, SchemaVersion)
	}

	for name, profile := range f.Profiles {
		if err := profile.validate(name); err != nil {
			return err
		}
	}

	return nil
}

func (p Profile) validate(profileName string) error {
	if p.Fix != nil && p.Fix.WithEnv != nil {
		return fmt.Errorf("profiles.%s.fix: withEnv applies only to check", profileName)
	}

	if p.Audit != nil {
		if err := p.Audit.validate(); err != nil {
			return fmt.Errorf("profiles.%s.audit: %w", profileName, err)
		}
	}

	if p.Run != nil {
		if err := p.Run.validate(profileName); err != nil {
			return err
		}
	}

	return nil
}

func (r RunBlock) validate(profileName string) error {
	for name, targets := range r.Scripts {
		if len(targets) == 0 {
			return fmt.Errorf("profiles.%s.run.scripts.%s: no targets defined", profileName, name)
		}

		for i, target := range targets {
			if err := target.Validate(); err != nil {
				if len(targets) == 1 {
					return fmt.Errorf("profiles.%s.run.scripts.%s: %w", profileName, name, err)
				}

				return fmt.Errorf("profiles.%s.run.scripts.%s[%d]: %w", profileName, name, i, err)
			}
		}
	}

	return nil
}

func (s ScriptTarget) Validate() error {
	count := 0

	if s.JS != "" {
		count++
	}

	if s.Composer != "" {
		count++
	}

	if s.Go != "" {
		count++
	}

	if s.Ruby != "" {
		count++
	}

	if s.Python != "" {
		count++
	}

	if s.Rust != "" {
		count++
	}

	switch count {
	case 0:
		return errors.New("set exactly one of js, composer, go, ruby, python, rust")
	case 1:
		return nil
	default:
		return errors.New("set only one of js, composer, go, ruby, python, rust per script")
	}
}

func (a AuditCommand) validate() error {
	if a.MinSeverity != nil && !isValidSeverity(*a.MinSeverity) {
		return fmt.Errorf("invalid minSeverity %q (use: info, low, moderate, medium, high, critical)", *a.MinSeverity)
	}

	return nil
}

func isValidSeverity(s string) bool {
	switch s {
	case "info", "low", "moderate", "medium", "high", "critical":
		return true
	default:
		return false
	}
}

func ResolveProfileName(cliProfile, envProfile, fileProfile string) string {
	if cliProfile != "" {
		return cliProfile
	}

	if envProfile != "" {
		return envProfile
	}

	if fileProfile != "" {
		return fileProfile
	}

	return "default"
}

func (f File) ProfileFor(name string) (Profile, error) {
	if len(f.Profiles) == 0 {
		return Profile{}, nil
	}

	profile, ok := f.Profiles[name]

	if !ok {
		return Profile{}, fmt.Errorf("unknown profile %q in preflight.yml", name)
	}

	return profile, nil
}
