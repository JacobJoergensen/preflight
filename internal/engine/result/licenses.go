package result

import (
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
)

type LicenseReport = Report[LicenseItem]

type LicenseItem struct {
	Project       string
	ScopeID       string
	ScopeDisplay  string
	Priority      int
	Inspected     int
	Violations    []LicenseViolation
	ErrText       string
	StartedAt     time.Time
	EndedAt       time.Time
	ElapsedMillis int64
}

type LicenseViolation struct {
	Package string
	Version string
	License string
	Reason  string
}

func FromLicenseResult(scopeID, scopeDisplay string, priority int, lr ecosystem.LicenseResult, violations []LicenseViolation, startedAt, endedAt time.Time) LicenseItem {
	item := LicenseItem{
		ScopeID:       scopeID,
		ScopeDisplay:  scopeDisplay,
		Priority:      priority,
		Inspected:     len(lr.Packages),
		Violations:    violations,
		StartedAt:     startedAt,
		EndedAt:       endedAt,
		ElapsedMillis: endedAt.Sub(startedAt).Milliseconds(),
	}

	if lr.Err != nil {
		item.ErrText = lr.Err.Error()
	}

	return item
}
