package engine

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs/memfs"
	"github.com/JacobJoergensen/preflight/internal/monorepo"
)

type fixRunner struct{}

func (fixRunner) Run(context.Context, string, ...string) (exec.Result, error) {
	return exec.Result{Stdout: "1.0.0"}, nil
}

func TestFixSingleProjectDryRun(t *testing.T) {
	r := Runner{
		FS:      memfs.New(map[string][]byte{"composer.json": nil}),
		Command: fixRunner{},
	}

	report, err := r.Fix(context.Background(), []string{"composer"}, ecosystem.FixOptions{DryRun: true}, false, AutoFixApprover{}, nil, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Plan) != 1 || report.Plan[0].ScopeID != "composer" {
		t.Fatalf("plan = %+v, want one composer entry", report.Plan)
	}

	if len(report.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(report.Items))
	}

	item := report.Items[0]

	if item.ScopeID != "composer" || !item.Success || item.WouldRun == "" {
		t.Errorf("item = %+v, want a successful composer dry-run with WouldRun set", item)
	}

	if report.BackupDir != "" {
		t.Errorf("BackupDir = %q, want empty on dry-run", report.BackupDir)
	}
}

func TestFixMonorepoDryRun(t *testing.T) {
	r := Runner{
		FS: memfs.New(map[string][]byte{
			filepath.Join("a", "composer.json"): nil,
			filepath.Join("b", "composer.json"): nil,
		}),
		Command: fixRunner{},
	}

	projects := []monorepo.Project{
		{AbsolutePath: "a", RelativePath: "pkg-a"},
		{AbsolutePath: "b", RelativePath: "pkg-b"},
	}

	report, err := r.runFix(context.Background(), projects, true, []string{"composer"}, ecosystem.FixOptions{DryRun: true}, false, AutoFixApprover{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Projects) != 2 {
		t.Fatalf("projects = %d, want 2", len(report.Projects))
	}

	if len(report.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(report.Items))
	}

	for _, item := range report.Items {
		if item.ScopeID != "composer" || !item.Success {
			t.Errorf("item = %+v, want a successful composer fix", item)
		}
	}
}
