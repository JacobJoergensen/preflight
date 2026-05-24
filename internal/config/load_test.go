package config

import (
	"strings"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/fs"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string][]byte
		wantError    bool
		errContain   string
		wantProfiles []string
	}{
		{
			name:      "missing file returns empty config",
			files:     map[string][]byte{},
			wantError: false,
		},
		{
			name: "invalid yaml returns error",
			files: map[string][]byte{
				"preflight.yml": []byte("{{invalid yaml"),
			},
			wantError:  true,
			errContain: "parse preflight.yml",
		},
		{
			name: "missing version returns error",
			files: map[string][]byte{
				"preflight.yml": []byte("profile: default\n"),
			},
			wantError:  true,
			errContain: "missing or invalid version",
		},
		{
			name: "unsupported version returns error",
			files: map[string][]byte{
				"preflight.yml": []byte("version: 99\n"),
			},
			wantError:  true,
			errContain: "unsupported version",
		},
		{
			name: "valid config loads",
			files: map[string][]byte{
				"preflight.yml": []byte(`version: 1
profile: default
profiles:
  default:
    check:
      scope: [js]
`),
			},
			wantError:    false,
			wantProfiles: []string{"default"},
		},
		{
			name: "validation error propagates",
			files: map[string][]byte{
				"preflight.yml": []byte(`version: 1
profiles:
  default:
    fix:
      withEnv: true
`),
			},
			wantError:  true,
			errContain: "withEnv applies only to check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := Load("", fs.NewMemFS(tt.files))

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
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantProfiles != nil {
				for _, name := range tt.wantProfiles {
					if _, ok := config.Profiles[name]; !ok {
						t.Errorf("expected profile %q not found", name)
					}
				}
			}
		})
	}
}
