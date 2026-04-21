package lockdiff

import (
	"cmp"
	"slices"

	"github.com/JacobJoergensen/preflight/internal/semver"
)

type ChangeKind string

const (
	ChangeAdded      ChangeKind = "added"
	ChangeRemoved    ChangeKind = "removed"
	ChangeUpgraded   ChangeKind = "upgraded"
	ChangeDowngraded ChangeKind = "downgraded"
)

type SemverLevel string

const (
	LevelMajor SemverLevel = "major"
	LevelMinor SemverLevel = "minor"
	LevelPatch SemverLevel = "patch"
	LevelOther SemverLevel = "other"
)

type PackageChange struct {
	Name    string
	Kind    ChangeKind
	FromVer string
	ToVer   string
	Level   SemverLevel
}

type FileDiff struct {
	File      string
	Ecosystem string
	Changes   []PackageChange
}

func (d FileDiff) Counts() (added, removed, upgraded, downgraded int) {
	for _, c := range d.Changes {
		switch c.Kind {
		case ChangeAdded:
			added++
		case ChangeRemoved:
			removed++
		case ChangeUpgraded:
			upgraded++
		case ChangeDowngraded:
			downgraded++
		}
	}

	return added, removed, upgraded, downgraded
}

func (d FileDiff) MajorUpgrades() int {
	var count int

	for _, c := range d.Changes {
		if c.Kind == ChangeUpgraded && c.Level == LevelMajor {
			count++
		}
	}

	return count
}

func (d FileDiff) Empty() bool {
	return len(d.Changes) == 0
}

func Diff(ecosystem, file string, before, after map[string]string) FileDiff {
	changes := make([]PackageChange, 0)

	for name, oldVersion := range before {
		newVersion, present := after[name]

		if !present {
			changes = append(changes, PackageChange{
				Name:    name,
				Kind:    ChangeRemoved,
				FromVer: oldVersion,
			})

			continue
		}

		if oldVersion == newVersion {
			continue
		}

		kind, level := classifyVersionChange(oldVersion, newVersion)
		changes = append(changes, PackageChange{
			Name:    name,
			Kind:    kind,
			FromVer: oldVersion,
			ToVer:   newVersion,
			Level:   level,
		})
	}

	for name, newVersion := range after {
		if _, existed := before[name]; existed {
			continue
		}

		changes = append(changes, PackageChange{
			Name:  name,
			Kind:  ChangeAdded,
			ToVer: newVersion,
		})
	}

	slices.SortFunc(changes, func(a, b PackageChange) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return FileDiff{
		File:      file,
		Ecosystem: ecosystem,
		Changes:   changes,
	}
}

func classifyVersionChange(oldVersion, newVersion string) (ChangeKind, SemverLevel) {
	oldParts := semver.ParseVersion(oldVersion)
	newParts := semver.ParseVersion(newVersion)

	if oldParts == nil || newParts == nil {
		if semver.Compare(oldVersion, newVersion) < 0 {
			return ChangeUpgraded, LevelOther
		}

		return ChangeDowngraded, LevelOther
	}

	kind := ChangeUpgraded

	if semver.Compare(oldVersion, newVersion) > 0 {
		kind = ChangeDowngraded
	}

	switch {
	case oldParts.Major != newParts.Major:
		return kind, LevelMajor
	case oldParts.Minor != newParts.Minor:
		return kind, LevelMinor
	case oldParts.Patch != newParts.Patch:
		return kind, LevelPatch
	default:
		return kind, LevelOther
	}
}
