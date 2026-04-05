package config

import (
	"errors"
	"fmt"
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
	Check *Command  `yaml:"check,omitempty"`
	Fix   *Command  `yaml:"fix,omitempty"`
	List  *Command  `yaml:"list,omitempty"`
	Run   *RunBlock `yaml:"run,omitempty"`
}

type RunBlock struct {
	Scripts map[string]ScriptTarget `yaml:"scripts,omitempty"`
}

type ScriptTarget struct {
	JS       string `yaml:"js,omitempty"`
	Composer string `yaml:"composer,omitempty"`
	Go       string `yaml:"go,omitempty"`
	Ruby     string `yaml:"ruby,omitempty"`
	Python   string `yaml:"python,omitempty"`
}

type Command struct {
	Scope   *[]string `yaml:"scope,omitempty"`
	PM      *[]string `yaml:"pm,omitempty"`
	WithEnv *bool     `yaml:"withEnv,omitempty"`
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

	if p.Run != nil {
		if err := p.Run.validate(profileName); err != nil {
			return err
		}
	}

	return nil
}

func (r RunBlock) validate(profileName string) error {
	for name, target := range r.Scripts {
		if err := target.Validate(); err != nil {
			return fmt.Errorf("profiles.%s.run.scripts.%s: %w", profileName, name, err)
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
