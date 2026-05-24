package ecosystem

import (
	"testing"

	"github.com/JacobJoergensen/preflight/internal/memfs"
	"github.com/JacobJoergensen/preflight/internal/model"
)

func TestMissingLockfileWarning(t *testing.T) {
	composer := Manager{
		Command:     "composer",
		ConfigFile:  "composer.json",
		LockFile:    "composer.lock",
		InstallArgs: []string{"install"},
	}

	tests := []struct {
		name     string
		manager  Manager
		present  map[string][]byte
		hasDeps  bool
		wantWarn bool
	}{
		{
			name:     "manifest present and lockfile missing with dependencies warns",
			manager:  composer,
			present:  map[string][]byte{"composer.json": nil},
			hasDeps:  true,
			wantWarn: true,
		},
		{
			name:     "present lockfile is silent",
			manager:  composer,
			present:  map[string][]byte{"composer.json": nil, "composer.lock": nil},
			hasDeps:  true,
			wantWarn: false,
		},
		{
			name:     "no dependencies is silent",
			manager:  composer,
			present:  map[string][]byte{"composer.json": nil},
			hasDeps:  false,
			wantWarn: false,
		},
		{
			name:     "manager without a lockfile is silent",
			manager:  Manager{Command: "pip", ConfigFile: "requirements.txt"},
			present:  map[string][]byte{"requirements.txt": nil},
			hasDeps:  true,
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := RunContext{FS: memfs.New(tt.present)}
			got := MissingLockfileWarning(rc, tt.manager, tt.hasDeps)

			if tt.wantWarn {
				if len(got) != 1 || got[0].Severity != model.SeverityWarning {
					t.Fatalf("expected one warning, got %v", got)
				}

				return
			}

			if len(got) != 0 {
				t.Fatalf("expected no messages, got %v", got)
			}
		})
	}
}
