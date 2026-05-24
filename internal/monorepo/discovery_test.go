package monorepo

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()

	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))

		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}

		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	return root
}

func projectNames(projects []Project) []string {
	names := make([]string, len(projects))

	for i, p := range projects {
		names[i] = p.Name
	}

	return names
}

func TestDiscoverCargoWorkspace(t *testing.T) {
	root := writeRepo(t, map[string]string{
		"Cargo.toml":          "[workspace]\nmembers = [\"crates/*\"]\n",
		"crates/a/Cargo.toml": "[package]\nname = \"crate-a\"\n",
		"crates/b/Cargo.toml": "[package]\nname = \"crate-b\"\n",
	})

	projects, err := DiscoverProjects(root)
	if err != nil {
		t.Fatalf("DiscoverProjects: %v", err)
	}

	if paths := projectPaths(projects); !slices.Equal(paths, []string{"crates/a", "crates/b"}) {
		t.Fatalf("paths = %v, want [crates/a crates/b]", paths)
	}

	if names := projectNames(projects); !slices.Equal(names, []string{"crate-a", "crate-b"}) {
		t.Errorf("names = %v, want [crate-a crate-b]", names)
	}
}

func TestDiscoverUvWorkspace(t *testing.T) {
	root := writeRepo(t, map[string]string{
		"pyproject.toml":            "[project]\nname = \"root\"\n\n[tool.uv.workspace]\nmembers = [\"packages/*\"]\n",
		"packages/x/pyproject.toml": "[project]\nname = \"pkg-x\"\n",
		"packages/y/pyproject.toml": "[project]\nname = \"pkg-y\"\n",
	})

	projects, err := DiscoverProjects(root)
	if err != nil {
		t.Fatalf("DiscoverProjects: %v", err)
	}

	if paths := projectPaths(projects); !slices.Equal(paths, []string{"packages/x", "packages/y"}) {
		t.Fatalf("paths = %v, want [packages/x packages/y]", paths)
	}

	if names := projectNames(projects); !slices.Equal(names, []string{"pkg-x", "pkg-y"}) {
		t.Errorf("names = %v, want [pkg-x pkg-y]", names)
	}
}

func TestDiscoverGoWork(t *testing.T) {
	root := writeRepo(t, map[string]string{
		"go.work":      "go 1.26\n\nuse (\n\t./svc-a\n\t./svc-b\n)\n",
		"svc-a/go.mod": "module example.com/a\n\ngo 1.26\n",
		"svc-b/go.mod": "module example.com/b\n\ngo 1.26\n",
	})

	projects, err := DiscoverProjects(root)
	if err != nil {
		t.Fatalf("DiscoverProjects: %v", err)
	}

	if paths := projectPaths(projects); !slices.Equal(paths, []string{"svc-a", "svc-b"}) {
		t.Fatalf("paths = %v, want [svc-a svc-b]", paths)
	}

	if names := projectNames(projects); !slices.Equal(names, []string{"example.com/a", "example.com/b"}) {
		t.Errorf("names = %v, want [example.com/a example.com/b]", names)
	}
}
