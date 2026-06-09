package ecosystem

import (
	"context"
	"strings"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/model"
)

func TestRunAudit(t *testing.T) {
	spec := &Spec{Name: "go"}

	t.Run("manager without an audit probe is unsupported", func(t *testing.T) {
		result := spec.RunAudit(context.Background(), RunContext{}, Detection{Active: Manager{Command: "go"}})

		if !result.Unsupported {
			t.Fatalf("expected unsupported, got %+v", result)
		}
	})

	t.Run("missing companion tool is skipped with its hint", func(t *testing.T) {
		manager := Manager{
			Command: "go",
			Audit: &AuditProbe{
				// A fabricated name guaranteed to be absent from PATH, so this exercises the
				// missing-tool branch without depending on any host-installed tool.
				Tool:            "preflight-nonexistent-audit-binary",
				Args:            []string{"-json"},
				ToolMissingHint: "install the audit tool",
				Parse:           func(string) []model.Finding { return nil },
			},
		}

		result := spec.RunAudit(context.Background(), RunContext{}, Detection{Active: manager})

		if !result.Skipped {
			t.Fatalf("expected skipped, got %+v", result)
		}

		if result.SkipReason != "install the audit tool" {
			t.Errorf("skip reason = %q, want the tool-missing hint", result.SkipReason)
		}
	})

	t.Run("missing lockfile is skipped with an install hint", func(t *testing.T) {
		manager := Manager{
			Command:     "npm",
			LockFile:    "package-lock.json",
			InstallArgs: []string{"install"},
			Audit: &AuditProbe{
				Args:  []string{"audit", "--json"},
				Parse: func(string) []model.Finding { return nil },
			},
		}

		rc := RunContext{FS: fs.NewMemFS(map[string][]byte{})}
		result := spec.RunAudit(context.Background(), rc, Detection{Active: manager})

		if !result.Skipped {
			t.Fatalf("expected skipped, got %+v", result)
		}

		if !strings.Contains(result.SkipReason, "npm install") {
			t.Errorf("skip reason = %q, want it to mention `npm install`", result.SkipReason)
		}
	})
}
