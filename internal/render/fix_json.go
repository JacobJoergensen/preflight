package render

import (
	"io"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/lockdiff"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

// FixJSONSchemaVersion is bumped when preflight fix --json output shape changes incompatibly.
const FixJSONSchemaVersion = 1

type JSONFixRenderer struct {
	Out io.Writer
}

type fixReportJSON struct {
	SchemaVersion int               `json:"schemaVersion"`
	StartedAt     time.Time         `json:"startedAt"`
	EndedAt       time.Time         `json:"endedAt"`
	Canceled      bool              `json:"canceled"`
	Aborted       bool              `json:"aborted,omitempty"`
	DryRun        bool              `json:"dryRun,omitempty"`
	SkipBackup    bool              `json:"skipBackup,omitempty"`
	Force         bool              `json:"force,omitempty"`
	BackupDir     string            `json:"backupDir,omitempty"`
	BackupDirs    map[string]string `json:"backupDirs,omitempty"`
	FixSelectors  []string          `json:"fixSelectors,omitempty"`
	Plan          []plannedFixJSON  `json:"plan,omitempty"`
	Items         []fixItemJSON     `json:"items"`
	Skipped       []skippedFixJSON  `json:"skipped,omitempty"`
	Diff          bool              `json:"diff,omitempty"`
	LockDiffs     []fileDiffJSON    `json:"lockDiffs,omitempty"`
	Projects      []projectJSON     `json:"projects,omitempty"`
}

type plannedFixJSON struct {
	Project     string `json:"project,omitempty"`
	ScopeID     string `json:"scopeId"`
	DisplayName string `json:"displayName"`
	Command     string `json:"command,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

type skippedFixJSON struct {
	Project     string `json:"project,omitempty"`
	ScopeID     string `json:"scopeId"`
	DisplayName string `json:"displayName,omitempty"`
	Command     string `json:"command,omitempty"`
	Reason      string `json:"reason"`
}

type fixItemJSON struct {
	Project        string     `json:"project,omitempty"`
	ScopeID        string     `json:"scopeId"`
	ManagerCommand string     `json:"managerCommand,omitempty"`
	ManagerName    string     `json:"managerName"`
	Version        string     `json:"version,omitempty"`
	Args           []string   `json:"args,omitempty"`
	WouldRun       string     `json:"wouldRun,omitempty"`
	Success        bool       `json:"success"`
	Error          string     `json:"error,omitempty"`
	Output         string     `json:"output,omitempty"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
}

type fileDiffJSON struct {
	File      string              `json:"file"`
	Ecosystem string              `json:"ecosystem"`
	Changes   []packageChangeJSON `json:"changes,omitempty"`
}

type packageChangeJSON struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	FromVersion string `json:"fromVersion,omitempty"`
	ToVersion   string `json:"toVersion,omitempty"`
	Level       string `json:"level,omitempty"`
}

func (r JSONFixRenderer) Render(report result.FixReport) error {
	if terminal.Quiet {
		return encodeJSON(r.Out, quietFixPayload(report), false)
	}

	return encodeJSON(r.Out, fixReportToJSON(report), true)
}

func fixReportToJSON(report result.FixReport) fixReportJSON {
	return fixReportJSON{
		SchemaVersion: FixJSONSchemaVersion,
		StartedAt:     report.StartedAt,
		EndedAt:       report.EndedAt,
		Canceled:      report.Canceled,
		Aborted:       report.Aborted,
		DryRun:        report.DryRun,
		SkipBackup:    report.SkipBackup,
		Force:         report.Force,
		BackupDir:     report.BackupDir,
		BackupDirs:    report.BackupDirs,
		FixSelectors:  report.FixSelectors,
		Plan:          plannedFixesToJSON(report.Plan),
		Items:         fixItemsToJSON(report.Items),
		Skipped:       skippedFixesToJSON(report.Skipped),
		Diff:          report.Diff,
		LockDiffs:     fileDiffsToJSON(report.LockDiffs),
		Projects:      projectsToJSON(report.Projects),
	}
}

func plannedFixesToJSON(planned []result.PlannedFix) []plannedFixJSON {
	if len(planned) == 0 {
		return nil
	}

	out := make([]plannedFixJSON, len(planned))

	for i, p := range planned {
		out[i] = plannedFixJSON{
			Project:     p.Project,
			ScopeID:     p.ScopeID,
			DisplayName: p.DisplayName,
			Command:     p.Command,
			Summary:     p.Summary,
		}
	}

	return out
}

func skippedFixesToJSON(skipped []result.SkippedFix) []skippedFixJSON {
	if len(skipped) == 0 {
		return nil
	}

	out := make([]skippedFixJSON, len(skipped))

	for i, s := range skipped {
		out[i] = skippedFixJSON{
			Project:     s.Project,
			ScopeID:     s.ScopeID,
			DisplayName: s.DisplayName,
			Command:     s.Command,
			Reason:      s.Reason,
		}
	}

	return out
}

func fixItemsToJSON(items []result.FixItem) []fixItemJSON {
	out := make([]fixItemJSON, 0, len(items))

	for _, item := range items {
		jsonItem := fixItemJSON{
			Project:        item.Project,
			ScopeID:        item.ScopeID,
			ManagerCommand: item.ManagerCommand,
			ManagerName:    item.ManagerName,
			Version:        item.Version,
			Args:           item.Args,
			WouldRun:       item.WouldRun,
			Success:        item.Success,
			Error:          item.Error,
			Output:         item.Output,
		}

		if !item.StartedAt.IsZero() {
			jsonItem.StartedAt = new(item.StartedAt)
		}

		if !item.EndedAt.IsZero() {
			jsonItem.EndedAt = new(item.EndedAt)
		}

		out = append(out, jsonItem)
	}

	return out
}

func fileDiffsToJSON(diffs []lockdiff.FileDiff) []fileDiffJSON {
	if len(diffs) == 0 {
		return nil
	}

	out := make([]fileDiffJSON, len(diffs))

	for i, d := range diffs {
		out[i] = fileDiffJSON{
			File:      d.File,
			Ecosystem: d.Ecosystem,
			Changes:   packageChangesToJSON(d.Changes),
		}
	}

	return out
}

func packageChangesToJSON(changes []lockdiff.PackageChange) []packageChangeJSON {
	if len(changes) == 0 {
		return nil
	}

	out := make([]packageChangeJSON, len(changes))

	for i, c := range changes {
		out[i] = packageChangeJSON{
			Name:        c.Name,
			Kind:        string(c.Kind),
			FromVersion: c.FromVer,
			ToVersion:   c.ToVer,
			Level:       string(c.Level),
		}
	}

	return out
}

func quietFixPayload(report result.FixReport) any {
	type quietItem struct {
		Project     string `json:"project,omitempty"`
		ScopeID     string `json:"scopeId"`
		ManagerName string `json:"managerName"`
		Error       string `json:"error,omitempty"`
	}

	type quietReport struct {
		Canceled bool        `json:"canceled"`
		Items    []quietItem `json:"items"`
	}

	items := make([]quietItem, 0, len(report.Items))

	for _, item := range report.Items {
		if item.Success {
			continue
		}

		items = append(items, quietItem{
			Project:     item.Project,
			ScopeID:     item.ScopeID,
			ManagerName: item.ManagerName,
			Error:       item.Error,
		})
	}

	return quietReport{
		Canceled: report.Canceled,
		Items:    items,
	}
}
