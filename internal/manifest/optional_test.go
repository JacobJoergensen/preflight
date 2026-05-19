package manifest

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePackageJSONOptionalDependencies(t *testing.T) {
	data := &PackageJSON{
		Dependencies:    map[string]string{"react": "^18"},
		DevDependencies: map[string]string{"vitest": "^1"},
		OptionalDependencies: map[string]string{
			"fsevents":    "^2",
			"node-sass":   "^9",
			"@scoped/lib": "^1",
		},
	}

	var config PackageConfig
	parsePackageJSON(&config, data)

	want := []string{"@scoped/lib", "fsevents", "node-sass"}

	if !reflect.DeepEqual(config.OptionalDependencies, want) {
		t.Errorf("OptionalDependencies = %v, want %v", config.OptionalDependencies, want)
	}
}

func TestParseComposerJSONSuggestStripsExtensions(t *testing.T) {
	data := &ComposerJSON{
		Require:    map[string]string{"php": "^8.3", "symfony/console": "^6"},
		RequireDev: map[string]string{"phpunit/phpunit": "^11"},
		Suggest: map[string]string{
			"ext-redis":         "For caching",
			"monolog/monolog":   "For logging",
			"symfony/messenger": "For async jobs",
		},
	}

	var config ComposerConfig
	parseComposerJSON(&config, data)

	want := []string{"monolog/monolog", "symfony/messenger"}

	if !reflect.DeepEqual(config.OptionalDependencies, want) {
		t.Errorf("OptionalDependencies = %v, want %v", config.OptionalDependencies, want)
	}
}

func TestParsePoetryDependenciesSection(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantRequired []string
		wantOptional []string
	}{
		{
			name: "string values are required",
			input: `[tool.poetry.dependencies]
python = "^3.11"
requests = "^2.31"
django = ">=4,<5"
`,
			wantRequired: []string{"requests", "django"},
			wantOptional: nil,
		},
		{
			name: "inline table with optional true",
			input: `[tool.poetry.dependencies]
torch = { version = "^2.0", optional = true }
boto3 = { version = "^1.0", optional = true, extras = ["s3"] }
`,
			wantRequired: nil,
			wantOptional: []string{"torch", "boto3"},
		},
		{
			name: "inline table without optional is required",
			input: `[tool.poetry.dependencies]
numpy = { version = "^1.24" }
sqlalchemy = { version = "^2.0", extras = ["asyncio"] }
`,
			wantRequired: []string{"numpy", "sqlalchemy"},
			wantOptional: nil,
		},
		{
			name: "explicit optional false stays required",
			input: `[tool.poetry.dependencies]
mocked = { version = "^1.0", optional = false }
`,
			wantRequired: []string{"mocked"},
			wantOptional: nil,
		},
		{
			name: "comments and blank lines are skipped",
			input: `[tool.poetry.dependencies]

# a comment
requests = "^2.31"  # inline comment
torch = { version = "^2.0", optional = true }
`,
			wantRequired: []string{"requests"},
			wantOptional: []string{"torch"},
		},
		{
			name: "optional inside a string value does not trigger",
			input: `[tool.poetry.dependencies]
weirdo = "optional = true"
`,
			wantRequired: []string{"weirdo"},
			wantOptional: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			required, optional := parsePoetryDependenciesSection(tc.input)

			if !reflect.DeepEqual(required, tc.wantRequired) {
				t.Errorf("required = %v, want %v", required, tc.wantRequired)
			}

			if !reflect.DeepEqual(optional, tc.wantOptional) {
				t.Errorf("optional = %v, want %v", optional, tc.wantOptional)
			}
		})
	}
}

func TestLoadPoetryPyprojectClassifiesOptional(t *testing.T) {
	tempDir := t.TempDir()

	pyproject := `[tool.poetry]
name = "demo"

[tool.poetry.dependencies]
python = "^3.11"
requests = "^2.31"
torch = { version = "^2.0", optional = true }
boto3 = { version = "^1.0", optional = true }

[tool.poetry.group.dev.dependencies]
pytest = "^8.0"
`

	if err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyproject), 0o600); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tempDir)
	main, dev, optional, requiresPython, err := loader.loadPoetryPyproject()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requiresPython != "^3.11" {
		t.Errorf("requiresPython = %q, want %q", requiresPython, "^3.11")
	}

	wantMain := []string{"requests"}

	if !reflect.DeepEqual(main, wantMain) {
		t.Errorf("main = %v, want %v", main, wantMain)
	}

	wantDev := []string{"pytest"}

	if !reflect.DeepEqual(dev, wantDev) {
		t.Errorf("dev = %v, want %v", dev, wantDev)
	}

	wantOptional := []string{"boto3", "torch"}

	if !reflect.DeepEqual(optional, wantOptional) {
		t.Errorf("optional = %v, want %v", optional, wantOptional)
	}
}

func TestLoadPEP621PyprojectSplitsDevAndOptionalExtras(t *testing.T) {
	tempDir := t.TempDir()

	pyproject := `[project]
name = "demo"
requires-python = ">=3.11"
dependencies = ["requests", "click"]

[project.optional-dependencies]
dev = ["pytest", "ruff"]
test = ["coverage"]
gpu = ["torch", "numpy"]
aws = ["boto3"]
`

	if err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyproject), 0o600); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tempDir)
	main, dev, optional, requiresPython, err := loader.loadPEP621Pyproject()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requiresPython != ">=3.11" {
		t.Errorf("requiresPython = %q, want %q", requiresPython, ">=3.11")
	}

	wantMain := []string{"click", "requests"}

	if !reflect.DeepEqual(main, wantMain) {
		t.Errorf("main = %v, want %v", main, wantMain)
	}

	wantDev := []string{"coverage", "pytest", "ruff"}

	if !reflect.DeepEqual(dev, wantDev) {
		t.Errorf("dev = %v, want %v", dev, wantDev)
	}

	wantOptional := []string{"boto3", "numpy", "torch"}

	if !reflect.DeepEqual(optional, wantOptional) {
		t.Errorf("optional = %v, want %v", optional, wantOptional)
	}
}
