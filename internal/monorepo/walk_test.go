package monorepo

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestDiscoverByWalkReturnsNilWhenFewerThanTwoProjects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files []string
	}{
		{
			name:  "empty directory",
			files: nil,
		},
		{
			name:  "only root has manifest",
			files: []string{"package.json"},
		},
		{
			name:  "only a single subproject",
			files: []string{"packages/ui/package.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := writeTestTree(t, tt.files)

			projects, err := discoverByWalk(root)

			if err != nil {
				t.Fatalf("discoverByWalk returned error: %v", err)
			}

			if projects != nil {
				t.Errorf("expected nil (single-project fallback), got %v", projectPaths(projects))
			}
		})
	}
}

func TestDiscoverByWalkFindsMultipleSubprojects(t *testing.T) {
	t.Parallel()

	root := writeTestTree(t, []string{
		"apps/web/package.json",
		"apps/api/go.mod",
		"packages/ui/package.json",
	})

	projects, err := discoverByWalk(root)

	if err != nil {
		t.Fatalf("discoverByWalk returned error: %v", err)
	}

	got := projectPaths(projects)
	sort.Strings(got)

	want := []string{"apps/api", "apps/web", "packages/ui"}

	if !stringSlicesEqual(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

func TestDiscoverByWalkIncludesRootWhenOtherSubprojectsPresent(t *testing.T) {
	t.Parallel()

	root := writeTestTree(t, []string{
		"go.mod",
		"services/auth/go.mod",
	})

	projects, err := discoverByWalk(root)

	if err != nil {
		t.Fatalf("discoverByWalk returned error: %v", err)
	}

	paths := projectPaths(projects)
	sort.Strings(paths)

	if !stringSlicesEqual(paths, []string{".", "services/auth"}) {
		t.Errorf("paths = %v, want [. services/auth]", paths)
	}
}

func TestDiscoverByWalkSkipsNoiseDirectories(t *testing.T) {
	t.Parallel()

	root := writeTestTree(t, []string{
		"apps/web/package.json",
		"apps/api/go.mod",
		"node_modules/lodash/package.json",
		"vendor/github.com/foo/bar/go.mod",
		".git/hooks/package.json",
		"target/build-output/go.mod",
	})

	projects, err := discoverByWalk(root)

	if err != nil {
		t.Fatalf("discoverByWalk returned error: %v", err)
	}

	paths := projectPaths(projects)
	sort.Strings(paths)

	want := []string{"apps/api", "apps/web"}

	if !stringSlicesEqual(paths, want) {
		t.Errorf("paths = %v, want %v (node_modules/vendor/.git/target should be skipped)", paths, want)
	}
}

func TestDiscoverByWalkIgnoresDotDirectories(t *testing.T) {
	t.Parallel()

	root := writeTestTree(t, []string{
		"apps/a/package.json",
		"apps/b/package.json",
		".preflight/backups/snapshot/package.json",
		".cache/build/go.mod",
	})

	projects, err := discoverByWalk(root)

	if err != nil {
		t.Fatalf("discoverByWalk returned error: %v", err)
	}

	for _, p := range projects {
		if strings.HasPrefix(p.RelativePath, ".") {
			t.Errorf("found dot-directory project %q in results", p.RelativePath)
		}
	}
}

func TestDiscoverByWalkRespectsMaxDepth(t *testing.T) {
	t.Parallel()

	deepPath := filepath.Join("a", "b", "c", "d", "e", "package.json")

	root := writeTestTree(t, []string{
		"apps/web/package.json",
		"apps/api/package.json",
		deepPath,
	})

	projects, err := discoverByWalk(root)

	if err != nil {
		t.Fatalf("discoverByWalk returned error: %v", err)
	}

	for _, p := range projects {
		if p.RelativePath == "a/b/c/d/e" {
			t.Errorf("project beyond walkMaxDepth was included: %q", p.RelativePath)
		}
	}
}

func writeTestTree(t *testing.T, files []string) string {
	t.Helper()

	root := t.TempDir()

	for _, rel := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))

		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}

		if err := os.WriteFile(full, []byte("{}"), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	return root
}

func projectPaths(projects []Project) []string {
	paths := make([]string, len(projects))

	for i, p := range projects {
		paths[i] = p.RelativePath
	}

	return paths
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
