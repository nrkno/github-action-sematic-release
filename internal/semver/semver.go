package semver

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// BumpType indicates what type of version bump is needed.
type BumpType int

const (
	BumpNone BumpType = iota
	BumpPatch
	BumpMinor
	BumpMajor
)

// Version represents a semantic version.
type Version struct {
	Major int
	Minor int
	Patch int
}

// String returns the version as "X.Y.Z".
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Tag returns the version as a git tag "vX.Y.Z".
//
// Deprecated: use FormatTagWithPrefix(v, "v") for configurable prefix support.
// This method hardcodes the "v" prefix and cannot reflect a custom tag-prefix.
func (v Version) Tag() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// ParseVersion parses a version string (e.g., "1.2.3" or "v1.2.3").
// Returns Version and error if parsing fails.
func ParseVersion(s string) (Version, error) {
	if s == "" {
		return Version{}, fmt.Errorf("empty version string")
	}

	// Use Masterminds/semver for parsing and validation
	parsed, err := semver.NewVersion(s)
	if err != nil {
		return Version{}, fmt.Errorf("invalid version: %w", err)
	}

	return Version{
		Major: int(parsed.Major()),
		Minor: int(parsed.Minor()),
		Patch: int(parsed.Patch()),
	}, nil
}

// NextVersion calculates the next version based on bump type.
// If bump is BumpNone, returns current version unchanged.
func NextVersion(current Version, bump BumpType) Version {
	switch bump {
	case BumpNone:
		return current
	case BumpPatch:
		return Version{
			Major: current.Major,
			Minor: current.Minor,
			Patch: current.Patch + 1,
		}
	case BumpMinor:
		return Version{
			Major: current.Major,
			Minor: current.Minor + 1,
			Patch: 0,
		}
	case BumpMajor:
		return Version{
			Major: current.Major + 1,
			Minor: 0,
			Patch: 0,
		}
	default:
		return current
	}
}

// String returns a human-readable name for the bump type.
func (b BumpType) String() string {
	switch b {
	case BumpMajor:
		return "major"
	case BumpMinor:
		return "minor"
	case BumpPatch:
		return "patch"
	default:
		return "none"
	}
}

// DetectBumpType analyzes commit types and returns the appropriate bump type.
// An optional rules map (map[string]string) overrides the built-in mapping.
// Built-in: "breaking-change"→BumpMajor, "feat"→BumpMinor, "fix"→BumpPatch.
//
// IMPORTANT: "BREAKING CHANGE" (with space) is NOT a trigger in the built-in
// switch. Callers must inject "breaking-change" (hyphenated) into commitTypes
// for commits with a "!" suffix or "BREAKING CHANGE:" footer.
//
// The variadic parameter accepts map[string]string (compatible with
// conventional.BumpRules = map[string]BumpLevel = map[string]string).
func DetectBumpType(commitTypes []string, rules ...map[string]string) BumpType {
	if len(rules) > 0 && len(rules[0]) > 0 {
		r := rules[0]
		highest := BumpNone
		for _, ct := range commitTypes {
			level, ok := r[ct]
			if !ok {
				continue
			}
			var b BumpType
			switch level {
			case "major":
				b = BumpMajor
			case "minor":
				b = BumpMinor
			case "patch":
				b = BumpPatch
			default:
				continue
			}
			if b > highest {
				highest = b
			}
		}
		return highest
	}

	// Built-in rules
	maxBump := BumpNone
	for _, ct := range commitTypes {
		var bump BumpType
		switch ct {
		case "breaking-change":
			bump = BumpMajor
		case "feat":
			bump = BumpMinor
		case "fix":
			bump = BumpPatch
		default:
			// Unrecognized type, skip
			continue
		}

		// Keep the highest bump
		if bump > maxBump {
			maxBump = bump
		}
	}

	return maxBump
}

// BootstrapVersion returns the initial version when no tags exist.
// First "feat" commit → v0.1.0
// First "fix" commit → v0.0.1
// "breaking-change" → v1.0.0
// No commits or unrecognized types → v0.0.0
func BootstrapVersion(commitTypes []string) Version {
	bump := DetectBumpType(commitTypes)
	// Start from v0.0.0 and apply the bump
	return NextVersion(Version{0, 0, 0}, bump)
}

// FormatTagWithPrefix formats a version as a git tag with the given prefix.
// prefix="v" → "v1.2.3"; prefix="" → "1.2.3"; prefix="release-" → "release-1.2.3".
// Use this instead of Version.Tag() when tag-prefix is configurable.
func FormatTagWithPrefix(v Version, prefix string) string {
	return prefix + v.String()
}

// ParseVersionFromTag strips the given prefix from tagName, then calls ParseVersion.
// Returns an error if tagName does not start with prefix.
// Example: ParseVersionFromTag("v1.2.3", "v") → Version{1,2,3}
//
//	ParseVersionFromTag("release-2.0.0", "release-") → Version{2,0,0}
//	ParseVersionFromTag("1.2.3", "") → Version{1,2,3}
func ParseVersionFromTag(tagName, prefix string) (Version, error) {
	if !strings.HasPrefix(tagName, prefix) {
		return Version{}, fmt.Errorf("tag %q does not start with prefix %q", tagName, prefix)
	}
	return ParseVersion(strings.TrimPrefix(tagName, prefix))
}

// FormatVersion formats a version for output (e.g., "1.2.3").
func FormatVersion(v Version) string {
	return v.String()
}

// FormatTag formats a version as a git tag (e.g., "v1.2.3").
func FormatTag(v Version) string {
	return v.Tag()
}
