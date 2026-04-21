package lockdiff

import (
	"slices"
	"testing"
)

func TestDiffClassifiesChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		before  map[string]string
		after   map[string]string
		wantKey string
		kind    ChangeKind
		level   SemverLevel
		from    string
		to      string
	}{
		{
			name:    "added package has no before version",
			before:  map[string]string{},
			after:   map[string]string{"lodash": "4.17.21"},
			wantKey: "lodash",
			kind:    ChangeAdded,
			to:      "4.17.21",
		},
		{
			name:    "removed package has no after version",
			before:  map[string]string{"lodash": "4.17.21"},
			after:   map[string]string{},
			wantKey: "lodash",
			kind:    ChangeRemoved,
			from:    "4.17.21",
		},
		{
			name:    "major version bump is major upgrade",
			before:  map[string]string{"react": "17.0.2"},
			after:   map[string]string{"react": "18.0.0"},
			wantKey: "react",
			kind:    ChangeUpgraded,
			level:   LevelMajor,
			from:    "17.0.2",
			to:      "18.0.0",
		},
		{
			name:    "minor version bump is minor upgrade",
			before:  map[string]string{"react": "18.1.0"},
			after:   map[string]string{"react": "18.2.0"},
			wantKey: "react",
			kind:    ChangeUpgraded,
			level:   LevelMinor,
			from:    "18.1.0",
			to:      "18.2.0",
		},
		{
			name:    "patch version bump is patch upgrade",
			before:  map[string]string{"react": "18.2.0"},
			after:   map[string]string{"react": "18.2.1"},
			wantKey: "react",
			kind:    ChangeUpgraded,
			level:   LevelPatch,
			from:    "18.2.0",
			to:      "18.2.1",
		},
		{
			name:    "reverse version change is downgrade",
			before:  map[string]string{"react": "18.2.0"},
			after:   map[string]string{"react": "18.1.0"},
			wantKey: "react",
			kind:    ChangeDowngraded,
			level:   LevelMinor,
			from:    "18.2.0",
			to:      "18.1.0",
		},
		{
			name:    "prerelease-only change falls back to other level",
			before:  map[string]string{"pkg": "1.0.0-alpha"},
			after:   map[string]string{"pkg": "1.0.0-beta"},
			wantKey: "pkg",
			kind:    ChangeUpgraded,
			level:   LevelOther,
			from:    "1.0.0-alpha",
			to:      "1.0.0-beta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := Diff("node", "package-lock.json", tt.before, tt.after)

			if len(diff.Changes) != 1 {
				t.Fatalf("got %d changes, want 1", len(diff.Changes))
			}

			got := diff.Changes[0]

			if got.Name != tt.wantKey {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantKey)
			}

			if got.Kind != tt.kind {
				t.Errorf("Kind = %q, want %q", got.Kind, tt.kind)
			}

			if got.Level != tt.level {
				t.Errorf("Level = %q, want %q", got.Level, tt.level)
			}

			if got.FromVer != tt.from {
				t.Errorf("FromVer = %q, want %q", got.FromVer, tt.from)
			}

			if got.ToVer != tt.to {
				t.Errorf("ToVer = %q, want %q", got.ToVer, tt.to)
			}
		})
	}
}

func TestDiffSkipsUnchangedPackages(t *testing.T) {
	t.Parallel()

	before := map[string]string{"lodash": "4.17.21", "react": "18.0.0"}
	after := map[string]string{"lodash": "4.17.21", "react": "18.0.0"}

	diff := Diff("node", "package-lock.json", before, after)

	if !diff.Empty() {
		t.Errorf("Empty() = false, want true for identical inputs; got %+v", diff.Changes)
	}
}

func TestDiffSortsChangesByName(t *testing.T) {
	t.Parallel()

	before := map[string]string{"zeta": "1.0.0", "alpha": "1.0.0"}
	after := map[string]string{"zeta": "2.0.0", "alpha": "2.0.0", "beta": "1.0.0"}

	diff := Diff("node", "package-lock.json", before, after)
	names := make([]string, 0, len(diff.Changes))

	for _, change := range diff.Changes {
		names = append(names, change.Name)
	}

	if !slices.IsSorted(names) {
		t.Errorf("changes not sorted by name: %v", names)
	}
}

func TestMajorUpgradesExcludesDowngradesAndLowerLevels(t *testing.T) {
	t.Parallel()

	before := map[string]string{
		"major-up":   "1.0.0",
		"major-down": "2.0.0",
		"minor-up":   "1.1.0",
	}
	after := map[string]string{
		"major-up":   "2.0.0",
		"major-down": "1.0.0",
		"minor-up":   "1.2.0",
	}

	diff := Diff("node", "package-lock.json", before, after)

	if got := diff.MajorUpgrades(); got != 1 {
		t.Errorf("MajorUpgrades() = %d, want 1 (only major-up qualifies)", got)
	}
}
