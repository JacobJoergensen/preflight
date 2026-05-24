package python

import (
	"maps"
	"slices"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/memfs"
)

func pyprojectContext(content string) ecosystem.RunContext {
	return ecosystem.RunContext{FS: memfs.New(map[string][]byte{"pyproject.toml": []byte(content)})}
}

func TestLoadPoetryPyproject(t *testing.T) {
	rc := pyprojectContext(`[tool.poetry.dependencies]
python = "^3.11"
requests = "^2.0"
rich = { version = "^13", optional = true }

[tool.poetry.group.dev.dependencies]
pytest = "^8.0"
`)

	main, dev, optional, requiresPython, err := loadPoetryPyproject(rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !slices.Equal(main, []string{"requests"}) {
		t.Errorf("main = %v, want [requests]", main)
	}

	if !slices.Equal(dev, []string{"pytest"}) {
		t.Errorf("dev = %v, want [pytest]", dev)
	}

	if !slices.Equal(optional, []string{"rich"}) {
		t.Errorf("optional = %v, want [rich]", optional)
	}

	if requiresPython != "^3.11" {
		t.Errorf("requiresPython = %q, want ^3.11", requiresPython)
	}
}

func TestLoadPEP621Pyproject(t *testing.T) {
	rc := pyprojectContext(`[project]
requires-python = ">=3.10"
dependencies = ["requests>=2.0", "click"]

[project.optional-dependencies]
dev = ["pytest>=8"]
docs = ["sphinx"]
web = ["flask"]
`)

	main, dev, optional, requiresPython, err := loadPEP621Pyproject(rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !slices.Equal(main, []string{"click", "requests"}) {
		t.Errorf("main = %v, want [click requests]", main)
	}

	if !slices.Equal(dev, []string{"pytest", "sphinx"}) {
		t.Errorf("dev = %v, want [pytest sphinx]", dev)
	}

	if !slices.Equal(optional, []string{"flask"}) {
		t.Errorf("optional = %v, want [flask]", optional)
	}

	if requiresPython != ">=3.10" {
		t.Errorf("requiresPython = %q, want >=3.10", requiresPython)
	}
}

func TestLoadPEP621PyprojectMissingProject(t *testing.T) {
	rc := pyprojectContext("[tool.uv]\ndev-dependencies = []\n")

	if _, _, _, _, err := loadPEP621Pyproject(rc); err == nil {
		t.Error("expected an error when [project] is absent")
	}
}

func TestLoadPipfileDeps(t *testing.T) {
	rc := ecosystem.RunContext{FS: memfs.New(map[string][]byte{"Pipfile": []byte(`[packages]
requests = "*"
flask = ">=2"

[dev-packages]
pytest = "*"
`)})}

	main, dev, err := loadPipfileDeps(rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !slices.Equal(main, []string{"flask", "requests"}) {
		t.Errorf("main = %v, want [flask requests]", main)
	}

	if !slices.Equal(dev, []string{"pytest"}) {
		t.Errorf("dev = %v, want [pytest]", dev)
	}
}

func TestParsePipListJSON(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   map[string]string
	}{
		{
			name:   "lowercases package names",
			output: `[{"name":"Django","version":"4.0"},{"name":"requests","version":"2.0"}]`,
			want:   map[string]string{"django": "4.0", "requests": "2.0"},
		},
		{
			name:   "skips entries with an empty name",
			output: `[{"name":"","version":"1.0"},{"name":"flask","version":"2.0"}]`,
			want:   map[string]string{"flask": "2.0"},
		},
		{
			name:   "invalid json yields an empty map",
			output: `not json`,
			want:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePipListJSON(tt.output); !maps.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
