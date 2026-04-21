package semver

import "testing"

func TestValidateVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		installed, required string
		valid               bool
	}{
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "^1.2.0", true},
		{"1.2.3", "^2.0.0", false},
		{"v1.2.3", "1.2.3", true},
	}

	for _, tt := range tests {
		valid, _ := ValidateVersion(tt.installed, tt.required)

		if valid != tt.valid {
			t.Errorf("ValidateVersion(%q, %q) = %v, want %v", tt.installed, tt.required, valid, tt.valid)
		}
	}
}

func TestMatchVersionConstraint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		installed, constraint string
		match                 bool
	}{
		// Exact match
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false},

		// Wildcards
		{"1.2.3", "*", true},
		{"1.2.3", "1.2.x", true},
		{"1.2.3", "1.3.x", false},

		// Greater than / less than
		{"1.2.3", ">=1.2.0", true},
		{"1.2.3", ">=1.3.0", false},
		{"1.2.3", ">1.2.2", true},
		{"1.2.3", "<1.3.0", true},
		{"1.2.3", "<=1.2.3", true},

		// Caret (npm-style)
		{"1.2.3", "^1.2.0", true},
		{"1.2.3", "^1.0.0", true},
		{"1.2.3", "^2.0.0", false},
		{"0.2.3", "^0.2.0", true},
		{"0.3.0", "^0.2.0", false},

		// Tilde (npm-style)
		{"1.2.3", "~1.2.0", true},
		{"1.3.0", "~1.2.0", false},
		{"1.2.5", "~1.2", true},

		// OR constraints
		{"1.2.3", "^1.0.0 || ^2.0.0", true},
		{"2.5.0", "^1.0.0 || ^2.0.0", true},
		{"3.0.0", "^1.0.0 || ^2.0.0", false},

		// AND constraints
		{"1.2.3", ">=1.0.0 <=2.0.0", true},
		{"1.2.3", ">=1.0.0 <1.2.0", false},

		// Hyphen ranges
		{"1.2.3", "1.0.0 - 2.0.0", true},
		{"0.9.0", "1.0.0 - 2.0.0", false},

		// Prerelease
		{"1.2.3-alpha", "1.2.3-alpha", true},
		{"1.2.3-beta", "1.2.3-alpha", false},
		{"1.2.3", ">=1.2.3-alpha", true},
	}

	for _, tt := range tests {
		if got := MatchVersionConstraint(tt.installed, tt.constraint); got != tt.match {
			t.Errorf("MatchVersionConstraint(%q, %q) = %v, want %v", tt.installed, tt.constraint, got, tt.match)
		}
	}
}

func TestMatchMinimumVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		installed, minimum string
		match              bool
	}{
		{"1.26.1", "1.26", true},
		{"1.26.0", "1.26", true},
		{"1.25.9", "1.26", false},
		{"20.1.0", "20", true},
		{"19.9.9", "20", false},
		{"1.26.1", "", true},
		{"v1.26.1", "1.26", true},
	}

	for _, tt := range tests {
		if got := MatchMinimumVersion(tt.installed, tt.minimum); got != tt.match {
			t.Errorf("MatchMinimumVersion(%q, %q) = %v, want %v", tt.installed, tt.minimum, got, tt.match)
		}
	}
}

func TestParseVersionPin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pin  string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"3.2.1", "3.2.1"},
		{"v3.2.1", "3.2.1"},
		{"3.2", "3.2"},
		{"v3.2", "3.2"},
		{"3", "3"},
		{"v3", "3"},
		{"20.10.0", "20.10.0"},
		{"lts/iron", "lts/iron"},
		{"lts/*", "lts/*"},
		{"system", "system"},
		{"  v3.2.1  ", "3.2.1"},
	}

	for _, tt := range tests {
		t.Run(tt.pin, func(t *testing.T) {
			if got := ParseVersionPin(tt.pin); got != tt.want {
				t.Errorf("ParseVersionPin(%q) = %q, want %q", tt.pin, got, tt.want)
			}
		})
	}
}

func TestMatchVersionConstraintCommaAnd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		installed, constraint string
		match                 bool
	}{
		{"1.5.0", ">=1.0.0,<2.0.0", true},
		{"2.0.0", ">=1.0.0,<2.0.0", false},
		{"0.9.0", ">=1.0.0,<2.0.0", false},
		{"1.2.3", ">=1.0.0,<=1.5.0", true},
		{"1.6.0", ">=1.0.0,<=1.5.0", false},
	}

	for _, tt := range tests {
		if got := MatchVersionConstraint(tt.installed, tt.constraint); got != tt.match {
			t.Errorf("MatchVersionConstraint(%q, %q) = %v, want %v", tt.installed, tt.constraint, got, tt.match)
		}
	}
}

func TestMatchVersionConstraintPessimistic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		installed, constraint string
		match                 bool
	}{
		// ~> with two parts acts like ^ (major can't change)
		{"two parts allows minor bump", "3.5.0", "~>3.2", true},
		{"two parts blocks major bump", "4.0.0", "~>3.2", false},
		{"two parts requires minimum", "3.1.0", "~>3.2", false},

		// ~> with three parts acts like ~ (minor can't change)
		{"three parts allows patch bump", "3.2.5", "~>3.2.1", true},
		{"three parts blocks minor bump", "3.3.0", "~>3.2.1", false},
		{"three parts requires minimum patch", "3.2.0", "~>3.2.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchVersionConstraint(tt.installed, tt.constraint); got != tt.match {
				t.Errorf("MatchVersionConstraint(%q, %q) = %v, want %v", tt.installed, tt.constraint, got, tt.match)
			}
		})
	}
}

func TestComparePrerelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pre1     string
		pre2     string
		wantSign int // -1, 0, or 1
	}{
		{"equal", "alpha", "alpha", 0},
		{"alpha before beta", "alpha", "beta", -1},
		{"numeric comparison", "1", "2", -1},
		{"numeric before string", "1", "alpha", -1},
		{"string after numeric", "alpha", "1", 1},
		{"dotted numeric", "alpha.1", "alpha.2", -1},
		{"longer wins when prefix equal", "alpha.1", "alpha.1.1", -1},
		{"rc1 before rc2", "rc.1", "rc.2", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := comparePrerelease(tt.pre1, tt.pre2)

			if (tt.wantSign < 0 && got >= 0) || (tt.wantSign > 0 && got <= 0) || (tt.wantSign == 0 && got != 0) {
				t.Errorf("comparePrerelease(%q, %q) = %d, want sign %d", tt.pre1, tt.pre2, got, tt.wantSign)
			}
		})
	}
}

func TestParseDetailedSemver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		major   int
		minor   int
		patch   int
		pre     string
		build   string
	}{
		{"1.2.3", 1, 2, 3, "", ""},
		{"1.2.3-alpha", 1, 2, 3, "alpha", ""},
		{"1.2.3+build", 1, 2, 3, "", "build"},
		{"1.2.3-alpha+build", 1, 2, 3, "alpha", "build"},
		{"1.2", 1, 2, -1, "", ""},
		{"1", 1, -1, -1, "", ""},
		{"0.0.0", 0, 0, 0, "", ""},
		{"1.2.3-beta.1", 1, 2, 3, "beta.1", ""},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := ParseVersion(tt.version)

			if got == nil {
				t.Fatal("ParseVersion returned nil")
			}

			if got.Major != tt.major {
				t.Errorf("Major = %d, want %d", got.Major, tt.major)
			}

			if got.Minor != tt.minor {
				t.Errorf("Minor = %d, want %d", got.Minor, tt.minor)
			}

			if got.Patch != tt.patch {
				t.Errorf("Patch = %d, want %d", got.Patch, tt.patch)
			}

			if got.Prerelease != tt.pre {
				t.Errorf("Prerelease = %q, want %q", got.Prerelease, tt.pre)
			}

			if got.Build != tt.build {
				t.Errorf("Build = %q, want %q", got.Build, tt.build)
			}
		})
	}
}

func TestParseDetailedSemverInvalid(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"",
		"abc",
		"not-a-version",
	}

	for _, version := range invalid {
		t.Run(version, func(t *testing.T) {
			if got := ParseVersion(version); got != nil {
				t.Errorf("ParseVersion(%q) = %+v, want nil", version, got)
			}
		})
	}
}
