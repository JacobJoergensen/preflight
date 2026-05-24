package ecosystem

import (
	"cmp"
	"context"
	goexec "os/exec"
	"slices"

	"github.com/JacobJoergensen/preflight/internal/exec"
)

func RunLicenseCommand(ctx context.Context, rc RunContext, tool, missingHint string, parse func(stdout string) []PackageLicense, args ...string) LicenseResult {
	if _, err := goexec.LookPath(tool); err != nil {
		return LicenseResult{Skipped: true, SkipReason: missingHint}
	}

	captured, err := exec.Capture(ctx, rc.WorkDir, tool, args...)
	if err != nil {
		return LicenseResult{Err: err}
	}

	return LicenseResult{Packages: parse(captured.Stdout)}
}

func SortPackageLicenses(packages []PackageLicense) {
	slices.SortFunc(packages, func(a, b PackageLicense) int {
		return cmp.Compare(a.Name, b.Name)
	})
}
