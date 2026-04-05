package config

import (
	"strings"
	"testing"
)

func TestScriptTargetValidate(t *testing.T) {
	tests := []struct {
		name       string
		target     ScriptTarget
		wantError  bool
		errContain string
	}{
		{
			name:       "empty target fails",
			target:     ScriptTarget{},
			wantError:  true,
			errContain: "set exactly one",
		},
		{
			name:      "single js target passes",
			target:    ScriptTarget{JS: "test"},
			wantError: false,
		},
		{
			name:      "single composer target passes",
			target:    ScriptTarget{Composer: "test"},
			wantError: false,
		},
		{
			name:      "single go target passes",
			target:    ScriptTarget{Go: "test"},
			wantError: false,
		},
		{
			name:      "single ruby target passes",
			target:    ScriptTarget{Ruby: "rake"},
			wantError: false,
		},
		{
			name:      "single python target passes",
			target:    ScriptTarget{Python: "pytest"},
			wantError: false,
		},
		{
			name:       "multiple targets fails",
			target:     ScriptTarget{JS: "test", Composer: "test"},
			wantError:  true,
			errContain: "set only one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target.Validate()

			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContain)
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveProfileName(t *testing.T) {
	tests := []struct {
		name        string
		cliProfile  string
		envProfile  string
		fileProfile string
		want        string
	}{
		{
			name: "all empty returns default",
			want: "default",
		},
		{
			name:        "file profile used when others empty",
			fileProfile: "production",
			want:        "production",
		},
		{
			name:        "env overrides file",
			envProfile:  "staging",
			fileProfile: "production",
			want:        "staging",
		},
		{
			name:        "cli overrides all",
			cliProfile:  "dev",
			envProfile:  "staging",
			fileProfile: "production",
			want:        "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveProfileName(tt.cliProfile, tt.envProfile, tt.fileProfile)

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFileProfileFor(t *testing.T) {
	tests := []struct {
		name      string
		file      File
		profile   string
		wantError bool
	}{
		{
			name:      "empty profiles returns empty profile",
			file:      File{},
			profile:   "default",
			wantError: false,
		},
		{
			name: "existing profile found",
			file: File{
				Profiles: map[string]Profile{
					"default": {Check: &Command{}},
				},
			},
			profile:   "default",
			wantError: false,
		},
		{
			name: "unknown profile returns error",
			file: File{
				Profiles: map[string]Profile{
					"default": {},
				},
			},
			profile:   "unknown",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.file.ProfileFor(tt.profile)

			if tt.wantError && err == nil {
				t.Fatal("expected error, got nil")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCommandValidate(t *testing.T) {
	tests := []struct {
		name      string
		command   Command
		wantError bool
	}{
		{
			name:      "empty command passes",
			command:   Command{},
			wantError: false,
		},
		{
			name:      "scope only passes",
			command:   Command{Scope: &[]string{"js"}},
			wantError: false,
		},
		{
			name:      "pm only passes",
			command:   Command{PM: &[]string{"npm"}},
			wantError: false,
		},
		{
			name:      "both scope and pm fails",
			command:   Command{Scope: &[]string{"js"}, PM: &[]string{"npm"}},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.command.validate("test")

			if tt.wantError && err == nil {
				t.Fatal("expected error, got nil")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestFileValidate(t *testing.T) {
	tests := []struct {
		name       string
		file       File
		wantError  bool
		errContain string
	}{
		{
			name:      "version 0 passes",
			file:      File{Version: 0},
			wantError: false,
		},
		{
			name:      "version 1 passes",
			file:      File{Version: 1},
			wantError: false,
		},
		{
			name:       "unsupported version fails",
			file:       File{Version: 99},
			wantError:  true,
			errContain: "unsupported version",
		},
		{
			name: "withEnv on fix fails",
			file: File{
				Version: 1,
				Profiles: map[string]Profile{
					"default": {Fix: &Command{WithEnv: new(true)}},
				},
			},
			wantError:  true,
			errContain: "withEnv applies only to check",
		},
		{
			name: "withEnv on list fails",
			file: File{
				Version: 1,
				Profiles: map[string]Profile{
					"default": {List: &Command{WithEnv: new(true)}},
				},
			},
			wantError:  true,
			errContain: "withEnv applies only to check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.file.validate()

			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContain)
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
