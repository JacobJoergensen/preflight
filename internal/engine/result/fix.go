package result

import (
	"slices"
	"time"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
)

type FixReport struct {
	StartedAt    time.Time
	EndedAt      time.Time
	Canceled     bool
	Aborted      bool
	DryRun       bool
	SkipBackup   bool
	BackupDir    string
	Force        bool
	FixSelectors []string
	Plan         []PlannedFix
	Items        []FixItem
	Skipped      []SkippedFix
	Diff         bool
	LockDiffs    []lockdiff.FileDiff
}

type PlannedFix struct {
	ScopeID     string
	DisplayName string
	Command     string
	Summary     string
}

type SkippedFix struct {
	ScopeID     string
	DisplayName string
	Command     string
	Reason      string
}

type FixItem struct {
	ScopeID        string
	ManagerCommand string
	ManagerName    string
	Version        string
	Args           []string
	WouldRun       string
	Success        bool
	Error          string
	StartedAt      time.Time
	EndedAt        time.Time
}

func FromAdapterFix(item adapter.FixItem, startedAt, endedAt time.Time) FixItem {
	return FixItem{
		ScopeID:        item.ScopeID,
		ManagerCommand: item.ManagerCommand,
		ManagerName:    item.ManagerName,
		Version:        item.Version,
		Args:           slices.Clone(item.Args),
		WouldRun:       item.WouldRun,
		Success:        item.Success,
		Error:          item.Error,
		StartedAt:      startedAt,
		EndedAt:        endedAt,
	}
}
