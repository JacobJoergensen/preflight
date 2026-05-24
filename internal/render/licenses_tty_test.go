package render

import (
	"testing"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
)

func TestLicenseStatusFromReport(t *testing.T) {
	clean := result.LicenseReport{Items: []result.LicenseItem{{ScopeID: "js", Inspected: 5}}}

	t.Run("no policy configured warns instead of passing", func(t *testing.T) {
		if _, _, text := licenseStatusFromReport(clean, false); text != noLicensePolicyText {
			t.Errorf("text = %q, want the no-policy hint", text)
		}
	})

	t.Run("policy configured and clean passes", func(t *testing.T) {
		if _, _, text := licenseStatusFromReport(clean, true); text != "All licenses comply with policy" {
			t.Errorf("text = %q, want the comply message", text)
		}
	})

	t.Run("violations take precedence over the no-policy hint", func(t *testing.T) {
		report := result.LicenseReport{Items: []result.LicenseItem{{ScopeID: "js", Violations: []result.LicenseViolation{{Package: "x"}}}}}

		if _, _, text := licenseStatusFromReport(report, false); text != "License policy violations found" {
			t.Errorf("text = %q, want the violations message", text)
		}
	})
}
