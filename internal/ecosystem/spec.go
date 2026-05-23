package ecosystem

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/model"
)

func (s *Spec) CanOutdated() bool {
	for _, manager := range s.Managers {
		if manager.Outdated != nil {
			return true
		}
	}

	return false
}

func (s *Spec) CanFix() bool {
	return len(s.Managers) > 0
}

func (s *Spec) Resolve(rc RunContext) (Detection, bool) {
	if s.AlwaysPresent {
		return Detection{}, true
	}

	markers := s.Detect

	if len(markers) == 0 {
		markers = s.defaultMarkers()
	}

	for _, marker := range markers {
		if marker.matches(rc) {
			return Detection{Active: s.activeManager(rc, marker.Manager)}, true
		}
	}

	return Detection{}, false
}

func (m Marker) matches(rc RunContext) bool {
	if !rc.FileExists(m.File) {
		return false
	}

	if m.Contains == "" {
		return true
	}

	data, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, m.File))

	return err == nil && strings.Contains(string(data), m.Contains)
}

func (s *Spec) activeManager(rc RunContext, fallback string) Manager {
	for _, manager := range s.Managers {
		if manager.LockFile != "" && rc.FileExists(manager.LockFile) {
			return manager
		}
	}

	for _, manager := range s.Managers {
		if manager.Command == fallback {
			return manager
		}
	}

	return Manager{}
}

func (s *Spec) defaultMarkers() []Marker {
	markers := make([]Marker, 0, len(s.Managers)*2)

	for _, manager := range s.Managers {
		if manager.LockFile != "" {
			markers = append(markers, Marker{File: manager.LockFile, Manager: manager.Command})
		}
	}

	for _, manager := range s.Managers {
		if manager.ConfigFile != "" {
			markers = append(markers, Marker{File: manager.ConfigFile, Manager: manager.Command})
		}
	}

	return markers
}

func (s *Spec) RunCheck(ctx context.Context, rc RunContext, detection Detection) []model.Message {
	if s.Check == nil {
		return nil
	}

	return s.Check(ctx, rc, detection)
}

func (s *Spec) RunFix(ctx context.Context, rc RunContext, detection Detection, options FixOptions) (FixItem, error) {
	manager := detection.Active

	item := FixItem{
		ScopeID:        s.Name,
		ManagerCommand: manager.Command,
		ManagerName:    managerName(manager),
	}

	versionResult, err := rc.Runner.Run(ctx, manager.Command, manager.VersionArgs...)
	if err != nil {
		item.Error = err.Error()
		return item, nil
	}

	item.Version = versionResult.Stdout

	args := slices.Clone(manager.InstallArgs)

	if options.Force {
		args = append(args, manager.ForceArgs...)
	}

	item.Args = slices.Clone(args)

	if options.DryRun {
		item.WouldRun = strings.TrimSpace(manager.Command + " " + strings.Join(args, " "))
		item.Success = true
		return item, nil
	}

	var captured bytes.Buffer

	writer := io.Writer(&captured)

	if _, err := rc.Stream.RunStreaming(ctx, manager.Command, args, writer, writer); err != nil {
		item.Error = err.Error()
		item.Output = captured.String()
		return item, nil
	}

	item.Success = true
	item.Output = captured.String()
	return item, nil
}

func (s *Spec) RunOutdated(ctx context.Context, rc RunContext, detection Detection) ([]OutdatedPackage, error) {
	probe := detection.Active.Outdated

	if probe == nil {
		return nil, nil
	}

	command := probe.Tool

	if command == "" {
		command = detection.Active.Command
	}

	result, err := rc.Runner.Run(ctx, command, probe.Args...)

	if err != nil && result.Stdout == "" {
		return nil, err
	}

	return probe.Parse(rc, result.Stdout)
}

func (s *Spec) Signals(rc RunContext, detection Detection) []string {
	if s.ExtraSignals != nil {
		return s.ExtraSignals(rc)
	}

	var lines []string

	if detection.Active.LockFile != "" && rc.FileExists(detection.Active.LockFile) {
		lines = append(lines, detection.Active.LockFile+" exists")
	}

	for _, pin := range s.VersionPins {
		if value := ReadVersionPin(rc, pin); value != "" {
			lines = append(lines, pin+" pins "+truncateSignal(value, signalPinMax))
		}
	}

	for _, name := range s.EnvSignals {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			lines = append(lines, "Shell: "+name+"="+shortenSignalPath(value, signalPathMax))
		}
	}

	return lines
}

func (s *Spec) FixPMHint(detection Detection) string {
	if !s.CanFix() {
		return ""
	}

	return detection.Active.Command
}

func (s *Spec) Title() string {
	if name := strings.TrimSpace(s.DisplayName); name != "" {
		return name
	}

	id := strings.TrimSpace(strings.ToLower(s.Name))

	if id == "" {
		return ""
	}

	return strings.ToUpper(id[:1]) + id[1:]
}

const (
	signalPinMax  = 48
	signalPathMax = 96
)

func truncateSignal(text string, limit int) string {
	if len(text) <= limit {
		return text
	}

	if limit <= 1 {
		return "…"
	}

	return text[:limit-1] + "…"
}

func shortenSignalPath(path string, limit int) string {
	if len(path) <= limit {
		return path
	}

	return "…" + path[len(path)-(limit-1):]
}

func managerName(manager Manager) string {
	if manager.DisplayName != "" {
		return manager.DisplayName
	}

	return manager.Command
}

func FilterDirect(packages []OutdatedPackage, direct map[string]struct{}) []OutdatedPackage {
	filtered := make([]OutdatedPackage, 0, len(packages))

	for _, pkg := range packages {
		if _, ok := direct[strings.ToLower(pkg.Name)]; ok {
			filtered = append(filtered, pkg)
		}
	}

	return filtered
}

func ToSet(lists ...[]string) map[string]struct{} {
	set := make(map[string]struct{})

	for _, list := range lists {
		for _, item := range list {
			set[strings.ToLower(item)] = struct{}{}
		}
	}

	return set
}
