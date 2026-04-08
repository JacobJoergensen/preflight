package config

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
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
	Check *Command      `yaml:"check,omitempty"`
	Fix   *Command      `yaml:"fix,omitempty"`
	List  *Command      `yaml:"list,omitempty"`
	Audit *AuditCommand `yaml:"audit,omitempty"`
	Run   *RunBlock     `yaml:"run,omitempty"`
}

type AuditCommand struct {
	Scope       *[]string `yaml:"scope,omitempty"`
	PM          *[]string `yaml:"pm,omitempty"`
	MinSeverity *string   `yaml:"minSeverity,omitempty"`
}

type RunBlock struct {
	Scripts map[string]ScriptTargets `yaml:"scripts,omitempty"`
}

type Command struct {
	Scope   *[]string `yaml:"scope,omitempty"`
	PM      *[]string `yaml:"pm,omitempty"`
	WithEnv *bool     `yaml:"withEnv,omitempty"`
}

type ScriptTarget struct {
	JS       string `yaml:"js,omitempty"`
	Composer string `yaml:"composer,omitempty"`
	Go       string `yaml:"go,omitempty"`
	Ruby     string `yaml:"ruby,omitempty"`
	Python   string `yaml:"python,omitempty"`
}

type ScriptTargets []ScriptTarget

func (s *ScriptTargets) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.MappingNode:
		var single ScriptTarget
		if err := node.Decode(&single); err != nil {
			return err
		}
		*s = []ScriptTarget{single}
	case yaml.SequenceNode:
		var multi []ScriptTarget
		if err := node.Decode(&multi); err != nil {
			return err
		}
		*s = multi
	default:
		return fmt.Errorf("expected map or sequence, got %v", node.Kind)
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
	if p.Check != nil {
		if err := p.Check.validate("check"); err != nil {
			return fmt.Errorf("profiles.%s.%w", profileName, err)
		}
	}

	if p.Fix != nil {
		if err := p.Fix.validate("fix"); err != nil {
			return fmt.Errorf("profiles.%s.%w", profileName, err)
		}

		if p.Fix.WithEnv != nil {
			return fmt.Errorf("profiles.%s.fix: withEnv applies only to check", profileName)
		}
	}

	if p.List != nil {
		if err := p.List.validate("list"); err != nil {
			return fmt.Errorf("profiles.%s.%w", profileName, err)
		}

		if p.List.WithEnv != nil {
			return fmt.Errorf("profiles.%s.list: withEnv applies only to check", profileName)
		}
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

	switch count {
	case 0:
		return errors.New("set exactly one of js, composer, go, ruby, python")
	case 1:
		return nil
	default:
		return errors.New("set only one of js, composer, go, ruby, python per script")
	}
}

func (c Command) validate(command string) error {
	if c.Scope != nil && c.PM != nil {
		return fmt.Errorf("%s: set only one of scope or pm", command)
	}

	return nil
}

func (a AuditCommand) validate() error {
	if a.Scope != nil && a.PM != nil {
		return errors.New("set only one of scope or pm")
	}

	if a.MinSeverity != nil {
		if !isValidSeverity(*a.MinSeverity) {
			return fmt.Errorf("invalid minSeverity %q (use: info, low, moderate, medium, high, critical)", *a.MinSeverity)
		}
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
