package php

import (
	"context"
	"strings"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/exec"
)

type fakeRunner struct {
	stdout string
	stderr string
}

func (f fakeRunner) Run(context.Context, string, ...string) (exec.Result, error) {
	return exec.Result{Stdout: f.stdout, Stderr: f.stderr}, nil
}

func TestFindPdoAlternative(t *testing.T) {
	tests := []struct {
		name    string
		ext     string
		sources map[string]string
		want    string
	}{
		{
			name:    "returns an installed pdo driver for pdo",
			ext:     "pdo",
			sources: map[string]string{"pdo_mysql": "core"},
			want:    "pdo_mysql",
		},
		{
			name:    "returns empty when no pdo driver is installed",
			ext:     "pdo",
			sources: map[string]string{"json": "core"},
			want:    "",
		},
		{
			name:    "returns empty for a non-split extension",
			ext:     "json",
			sources: map[string]string{"pdo_mysql": "core"},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findPdoAlternative(tt.ext, tt.sources); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPhpRuntimeVersionSkipsStartupWarnings(t *testing.T) {
	stdout := "Warning: PHP Startup: Unable to load dynamic library 'openssl' (tried: C:\\php\\ext\\openssl) in Unknown on line 0\n" +
		"PHP 8.5.1 (cli) (built: Dec 16 2025 16:25:44) (ZTS Visual C++ 2022 x64)\n" +
		"Copyright (c) The PHP Group"

	rc := ecosystem.RunContext{Runner: fakeRunner{stdout: stdout}}

	version, err := phpRuntimeVersion(context.Background(), rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if version != "8.5.1" {
		t.Errorf("version: got %q, want %q", version, "8.5.1")
	}
}

func TestPhpRuntimeVersionFindsBannerOnStderr(t *testing.T) {
	stdout := "Warning: PHP Startup: Unable to load dynamic library 'curl' (tried: C:\\php\\ext\\php_curl.dll (Access is denied)) in Unknown on line 0"
	stderr := "PHP 8.5.1 (cli) (built: Dec 16 2025 16:25:44) (ZTS Visual C++ 2022 x64)\n" +
		"Copyright (c) The PHP Group"

	rc := ecosystem.RunContext{Runner: fakeRunner{stdout: stdout, stderr: stderr}}

	version, err := phpRuntimeVersion(context.Background(), rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if version != "8.5.1" {
		t.Errorf("version: got %q, want %q", version, "8.5.1")
	}
}

func TestPhpExtensionsSkipsStartupWarnings(t *testing.T) {
	stdout := "[PHP Modules]\n" +
		"Warning: PHP Startup: Unable to load dynamic library 'openssl' (tried: C:\\php\\ext\\openssl) in Unknown on line 0\n" +
		"curl\n" +
		"Zend OPcache\n"

	rc := ecosystem.RunContext{Runner: fakeRunner{stdout: stdout}}

	extensions, err := phpExtensions(context.Background(), rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := extensions["curl"]; !ok {
		t.Errorf("expected curl to be detected")
	}

	if _, ok := extensions["Zend OPcache"]; !ok {
		t.Errorf("expected Zend OPcache to be detected")
	}

	for name := range extensions {
		if strings.Contains(name, "Warning") {
			t.Errorf("warning line registered as extension: %q", name)
		}
	}
}
