package semver

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	semverRegex = regexp.MustCompile(`(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([\w.-]+))?(?:\+([\w.-]+))?`)
	numberRegex = regexp.MustCompile(`[0-9]+`)
)

type VersionParts struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

func ValidateVersion(installedVersion, requiredVersion string) (bool, string) {
	installedVersion = strings.TrimPrefix(installedVersion, "v")

	if !MatchVersionConstraint(installedVersion, requiredVersion) {
		return false, fmt.Sprintf("Version %s is required, but version %s is installed.", requiredVersion, installedVersion)
	}

	return true, ""
}

func ParseVersionPin(pin string) string {
	pin = strings.TrimSpace(pin)

	if pin == "" {
		return ""
	}

	pin = strings.TrimPrefix(pin, "v")

	parts := parseDetailedSemver(pin)

	if parts == nil || parts.Major < 0 {
		return pin
	}

	if parts.Minor < 0 {
		return strconv.Itoa(parts.Major)
	}

	if parts.Patch < 0 {
		return fmt.Sprintf("%d.%d", parts.Major, parts.Minor)
	}

	return fmt.Sprintf("%d.%d.%d", parts.Major, parts.Minor, parts.Patch)
}

// MatchVersionConstraint supported operators: >=, >, <=, <, ^, ~, wildcards (*, x), OR (||), AND (space or comma), hyphen ranges
func MatchVersionConstraint(installed, required string) bool {
	required = strings.TrimSpace(required)

	if required == "" || required == "*" || required == "x" {
		return true
	}

	// OR constraints: any match is enough
	if strings.Contains(required, "||") {
		return matchOrConstraint(installed, required)
	}

	if strings.Contains(required, ",") {
		return matchCommaAndConstraint(installed, required)
	}

	// AND constraints with space (but not hyphen ranges)
	if strings.Contains(required, " ") && !strings.Contains(required, " - ") {
		return matchSpaceAndConstraint(installed, required)
	}

	return matchSingleConstraint(installed, required)
}

func MatchMinimumVersion(installed, minimum string) bool {
	minimum = strings.TrimSpace(minimum)

	if minimum == "" {
		return true
	}

	return MatchVersionConstraint(installed, ">="+minimum)
}

func matchOrConstraint(installed, required string) bool {
	for part := range strings.SplitSeq(required, "||") {
		if MatchVersionConstraint(installed, part) {
			return true
		}
	}

	return false
}

func matchCommaAndConstraint(installed, required string) bool {
	for part := range strings.SplitSeq(required, ",") {
		if !MatchVersionConstraint(installed, part) {
			return false
		}
	}

	return true
}

func matchSpaceAndConstraint(installed, required string) bool {
	parts := splitSpaceConstraints(required)

	for _, part := range parts {
		if !MatchVersionConstraint(installed, part) {
			return false
		}
	}

	return true
}

func splitSpaceConstraints(required string) []string {
	var parts []string
	var current strings.Builder

	operators := "><=^~"

	for i := 0; i < len(required); i++ {
		if required[i] == ' ' {
			j := i + 1

			for j < len(required) && required[j] == ' ' {
				j++
			}

			if j < len(required) && strings.ContainsRune(operators, rune(required[j])) {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}

				i = j - 1

				continue
			}
		}

		current.WriteByte(required[i])
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func matchSingleConstraint(installed, required string) bool {
	required = strings.TrimSpace(required)

	switch {
	case strings.HasPrefix(required, ">="):
		return compareVersions(installed, strings.TrimSpace(required[2:])) >= 0
	case strings.HasPrefix(required, "<="):
		return compareVersions(installed, strings.TrimSpace(required[2:])) <= 0
	case strings.HasPrefix(required, ">"):
		return compareVersions(installed, strings.TrimSpace(required[1:])) > 0
	case strings.HasPrefix(required, "<"):
		return compareVersions(installed, strings.TrimSpace(required[1:])) < 0
	case strings.HasPrefix(required, "^"):
		return matchCaretRange(installed, strings.TrimSpace(required[1:]))
	case strings.HasPrefix(required, "~>"):
		return matchPessimisticRange(installed, strings.TrimSpace(required[2:]))
	case strings.HasPrefix(required, "~"):
		return matchTildeRange(installed, strings.TrimSpace(required[1:]))
	case strings.Contains(required, " - "):
		return matchHyphenRange(installed, required)
	case strings.HasSuffix(required, ".*") || strings.HasSuffix(required, ".x"):
		return matchWildcard(installed, required)
	default:
		return compareVersions(installed, required) == 0
	}
}

func matchWildcard(installed, pattern string) bool {
	pattern = strings.TrimSuffix(pattern, ".*")
	pattern = strings.TrimSuffix(pattern, ".x")

	installedParts := parseDetailedSemver(installed)
	patternParts := parseDetailedSemver(pattern)

	if installedParts == nil || patternParts == nil {
		return strings.HasPrefix(installed, pattern)
	}

	if installedParts.Major != patternParts.Major {
		return false
	}

	if patternParts.Minor != -1 && installedParts.Minor != patternParts.Minor {
		return false
	}

	return true
}

func matchHyphenRange(installed, rangeStr string) bool {
	minVersion, maxVersion, found := strings.Cut(rangeStr, " - ")

	if !found {
		return false
	}

	return compareVersions(installed, strings.TrimSpace(minVersion)) >= 0 &&
		compareVersions(installed, strings.TrimSpace(maxVersion)) <= 0
}

// matchCaretRange implements the ^ operator from NPM's semver.
// ^1.2.3 -> >=1.2.3 <2.0.0
func matchCaretRange(installed, required string) bool {
	installedParts := parseDetailedSemver(installed)
	requiredParts := parseDetailedSemver(required)

	if installedParts == nil || requiredParts == nil {
		return installed == required
	}

	if installedParts.Major != requiredParts.Major {
		// For version 0.x.x, ^ means changes only allowed in patch level
		if requiredParts.Major == 0 && requiredParts.Minor > 0 {
			if installedParts.Major > 0 || installedParts.Minor > requiredParts.Minor {
				return false
			}

			return installedParts.Minor == requiredParts.Minor && installedParts.Patch >= requiredParts.Patch
		}

		// For versions 0.0.x, ^ means no changes allowed
		if requiredParts.Major == 0 && requiredParts.Minor == 0 {
			return installedParts.Major == 0 && installedParts.Minor == 0 &&
				installedParts.Patch == requiredParts.Patch
		}

		return false
	}

	if requiredParts.Major > 0 {
		return compareVersions(installed, required) >= 0 && installedParts.Major == requiredParts.Major
	}

	// For 0.y.z versions, ^ means the same as ~
	return matchTildeRange(installed, required)
}

// matchTildeRange implements the ~ operator from NPM's semver.
// ~1.2.3 -> >=1.2.3 <1.3.0
// ~1.2 -> >=1.2.0 <1.3.0
func matchTildeRange(installed, required string) bool {
	requiredParts := parseDetailedSemver(required)
	installedParts := parseDetailedSemver(installed)

	if requiredParts == nil || installedParts == nil {
		return installed == required
	}

	if installedParts.Major != requiredParts.Major {
		return false
	}

	if requiredParts.Minor != -1 && installedParts.Minor != requiredParts.Minor {
		return false
	}

	if requiredParts.Patch != -1 {
		if installedParts.Minor == requiredParts.Minor {
			return installedParts.Patch >= requiredParts.Patch
		}

		// If minor is higher, any patch version is ok
		return installedParts.Minor > requiredParts.Minor
	}

	return true
}

// matchPessimisticRange implements Ruby's ~> operator.
// ~> 3.2 -> >= 3.2.0, < 4.0.0 (acts like ^)
// ~> 3.2.1 -> >= 3.2.1, < 3.3.0 (acts like ~)
func matchPessimisticRange(installed, required string) bool {
	parts := strings.Split(required, ".")

	if len(parts) <= 2 {
		return matchCaretRange(installed, required)
	}

	return matchTildeRange(installed, required)
}

func compareVersions(v1, v2 string) int {
	parts1 := parseDetailedSemver(v1)
	parts2 := parseDetailedSemver(v2)

	if parts1 == nil || parts2 == nil {
		v1Parts, v2Parts := parseSemver(v1), parseSemver(v2)
		return compareVersionArrays(v1Parts, v2Parts)
	}

	if parts1.Major != parts2.Major {
		return cmp.Compare(parts1.Major, parts2.Major)
	}

	if parts1.Minor != parts2.Minor {
		return cmp.Compare(parts1.Minor, parts2.Minor)
	}

	if parts1.Patch != parts2.Patch {
		return cmp.Compare(parts1.Patch, parts2.Patch)
	}

	// Prerelease is lower than no prerelease
	if parts1.Prerelease == "" && parts2.Prerelease != "" {
		return 1
	}

	if parts1.Prerelease != "" && parts2.Prerelease == "" {
		return -1
	}

	return comparePrerelease(parts1.Prerelease, parts2.Prerelease)
}

func comparePrerelease(pre1, pre2 string) int {
	if pre1 == pre2 {
		return 0
	}

	ids1 := strings.Split(pre1, ".")
	ids2 := strings.Split(pre2, ".")
	minLen := min(len(ids1), len(ids2))

	for i := range minLen {
		if result := comparePrereleaseID(ids1[i], ids2[i]); result != 0 {
			return result
		}
	}

	// Larger set has higher precedence
	return cmp.Compare(len(ids1), len(ids2))
}

// comparePrereleaseID Numeric identifiers are compared as integers and have lower precedence than non-numeric.
func comparePrereleaseID(id1, id2 string) int {
	num1, err1 := strconv.Atoi(id1)
	num2, err2 := strconv.Atoi(id2)
	isNum1, isNum2 := err1 == nil, err2 == nil

	switch {
	case isNum1 && isNum2:
		return cmp.Compare(num1, num2)
	case isNum1:
		return -1
	case isNum2:
		return 1
	default:
		return strings.Compare(id1, id2)
	}
}

func compareVersionArrays(v1Parts, v2Parts []int) int {
	maxLen := max(len(v1Parts), len(v2Parts))

	v1Extended := make([]int, maxLen)
	v2Extended := make([]int, maxLen)

	copy(v1Extended, v1Parts)
	copy(v2Extended, v2Parts)

	for i := range v1Extended {
		if v1Extended[i] != v2Extended[i] {
			return cmp.Compare(v1Extended[i], v2Extended[i])
		}
	}

	return 0
}

func parseDetailedSemver(version string) *VersionParts {
	matches := semverRegex.FindStringSubmatch(version)

	if len(matches) < 4 {
		return nil
	}

	result := &VersionParts{
		Major:      -1,
		Minor:      -1,
		Patch:      -1,
		Prerelease: "",
		Build:      "",
	}

	if major, err := strconv.Atoi(matches[1]); err == nil {
		result.Major = major
	} else {
		return nil
	}

	if matches[2] != "" {
		if minor, err := strconv.Atoi(matches[2]); err == nil {
			result.Minor = minor
		}
	}

	if matches[3] != "" {
		if patch, err := strconv.Atoi(matches[3]); err == nil {
			result.Patch = patch
		}
	}

	if len(matches) >= 5 && matches[4] != "" {
		result.Prerelease = matches[4]
	}

	if len(matches) >= 6 && matches[5] != "" {
		result.Build = matches[5]
	}

	return result
}

func parseSemver(version string) []int {
	parts := numberRegex.FindAllString(version, -1)
	parsed := make([]int, 0, len(parts))

	for _, part := range parts {
		if number, err := strconv.Atoi(part); err == nil {
			parsed = append(parsed, number)
		}
	}

	return parsed
}
