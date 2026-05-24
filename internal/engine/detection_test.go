package engine

import (
	"testing"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/fs/memfs"
)

func TestSpecResolveDetection(t *testing.T) {
	tests := []struct {
		name        string
		scope       string
		files       map[string][]byte
		wantPresent bool
		wantActive  string
	}{
		{"go via go.mod", "go", map[string][]byte{"go.mod": nil}, true, "go"},
		{"go via go.sum and go.mod", "go", map[string][]byte{"go.sum": nil, "go.mod": nil}, true, "go"},
		{"go absent", "go", nil, false, ""},

		{"rust via Cargo.toml", "rust", map[string][]byte{"Cargo.toml": nil}, true, "cargo"},
		{"rust absent", "rust", nil, false, ""},

		{"composer via composer.json", "composer", map[string][]byte{"composer.json": nil}, true, "composer"},
		{"composer absent", "composer", nil, false, ""},

		{"js package.json defaults to npm", "js", map[string][]byte{"package.json": nil}, true, "npm"},
		{"js bun.lock selects bun", "js", map[string][]byte{"package.json": nil, "bun.lock": nil}, true, "bun"},
		{"js pnpm-lock selects pnpm", "js", map[string][]byte{"package.json": nil, "pnpm-lock.yaml": nil}, true, "pnpm"},
		{"js yarn.lock selects yarn", "js", map[string][]byte{"package.json": nil, "yarn.lock": nil}, true, "yarn"},
		{"js package-lock selects npm", "js", map[string][]byte{"package.json": nil, "package-lock.json": nil}, true, "npm"},
		{"js package-lock and yarn.lock prefers yarn", "js", map[string][]byte{"package.json": nil, "package-lock.json": nil, "yarn.lock": nil}, true, "yarn"},
		{"js node_modules without manifest defaults to npm", "js", map[string][]byte{"node_modules": nil}, true, "npm"},
		{"js lockfile without manifest is absent", "js", map[string][]byte{"yarn.lock": nil}, false, ""},
		{"js absent", "js", nil, false, ""},

		{"python requirements selects pip", "python", map[string][]byte{"requirements.txt": nil}, true, "pip"},
		{"python poetry.lock selects poetry", "python", map[string][]byte{"poetry.lock": nil, "pyproject.toml": []byte("[tool.poetry]")}, true, "poetry"},
		{"python tool.poetry selects poetry", "python", map[string][]byte{"pyproject.toml": []byte("[tool.poetry]\n")}, true, "poetry"},
		{"python tool.pdm with project selects pdm", "python", map[string][]byte{"pyproject.toml": []byte("[project]\n[tool.pdm]\n")}, true, "pdm"},
		{"python project only selects uv", "python", map[string][]byte{"pyproject.toml": []byte("[project]\n")}, true, "uv"},
		{"python requirements wins over tool marker", "python", map[string][]byte{"requirements.txt": nil, "pyproject.toml": []byte("[tool.poetry]")}, true, "pip"},
		{"python uv.lock wins over tool.poetry", "python", map[string][]byte{"uv.lock": nil, "pyproject.toml": []byte("[tool.poetry]")}, true, "uv"},
		{"python Pipfile selects pipenv", "python", map[string][]byte{"Pipfile": nil}, true, "pipenv"},
		{"python absent", "python", nil, false, ""},

		{"ruby Gemfile selects bundle", "ruby", map[string][]byte{"Gemfile": nil}, true, "bundle"},
		{"ruby gems.rb selects bundle", "ruby", map[string][]byte{"gems.rb": nil}, true, "bundle"},
		{"ruby lockfile without manifest is absent", "ruby", map[string][]byte{"Gemfile.lock": nil}, false, ""},
		{"ruby absent", "ruby", nil, false, ""},

		{"php via composer.json", "php", map[string][]byte{"composer.json": nil}, true, ""},
		{"php via composer.lock", "php", map[string][]byte{"composer.lock": nil}, true, ""},
		{"php absent", "php", nil, false, ""},

		{"node via package.json", "node", map[string][]byte{"package.json": nil}, true, ""},
		{"node absent", "node", nil, false, ""},

		{"env always present", "env", nil, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := ecosystem.Lookup(tt.scope)
			if !ok {
				t.Fatalf("no spec registered for scope %q", tt.scope)
			}

			rc := ecosystem.RunContext{FS: memfs.New(tt.files)}
			detection, present := spec.Resolve(rc)

			if present != tt.wantPresent {
				t.Fatalf("present = %v, want %v", present, tt.wantPresent)
			}

			if present && detection.Active.Command != tt.wantActive {
				t.Errorf("active manager = %q, want %q", detection.Active.Command, tt.wantActive)
			}
		})
	}
}
