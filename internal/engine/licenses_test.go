package engine

import (
	"testing"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
)

func TestEvaluateLicenses(t *testing.T) {
	packages := []ecosystem.PackageLicense{
		{Name: "a", Version: "1", License: "MIT"},
		{Name: "b", Version: "2", License: "GPL-3.0-only"},
		{Name: "c", Version: "3", License: "MIT OR Apache-2.0"},
		{Name: "d", Version: "4", License: ""},
	}

	t.Run("deny matches a token case-insensitively", func(t *testing.T) {
		violations := evaluateLicenses(packages, newLicensePolicy(nil, []string{"gpl-3.0-only"}))

		if len(violations) != 1 || violations[0].Package != "b" {
			t.Fatalf("got %+v, want only b denied", violations)
		}
	})

	t.Run("allowlist permits a satisfied OR token and skips empty", func(t *testing.T) {
		violations := evaluateLicenses(packages, newLicensePolicy([]string{"MIT"}, nil))

		// a (MIT) and c (MIT OR Apache) satisfy the allowlist, d (no license) is
		// skipped, only b (GPL) is left.
		if len(violations) != 1 || violations[0].Package != "b" {
			t.Fatalf("got %+v, want only b violating the allowlist", violations)
		}
	})

	t.Run("no policy yields no violations", func(t *testing.T) {
		if violations := evaluateLicenses(packages, newLicensePolicy(nil, nil)); violations != nil {
			t.Fatalf("got %+v, want none", violations)
		}
	})
}
