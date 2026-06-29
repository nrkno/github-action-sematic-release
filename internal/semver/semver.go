package semver

import (
	"fmt"

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

// DetectBumpType analyzes commit types and returns the appropriate bump type.
// Types: "feat" → BumpMinor, "fix" → BumpPatch, "BREAKING CHANGE" → BumpMajor
// If multiple types present, highest bump wins (Major > Minor > Patch).
// If no recognized types, returns BumpNone.
func DetectBumpType(commitTypes []string) BumpType {
	maxBump := BumpNone

	for _, ct := range commitTypes {
		var bump BumpType
		switch ct {
		case "BREAKING CHANGE":
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
// "BREAKING CHANGE" → v1.0.0
// No commits or unrecognized types → v0.0.0
func BootstrapVersion(commitTypes []string) Version {
	bump := DetectBumpType(commitTypes)
	// Start from v0.0.0 and apply the bump
	return NextVersion(Version{0, 0, 0}, bump)
}

// FormatVersion formats a version for output (e.g., "1.2.3").
func FormatVersion(v Version) string {
	return v.String()
}

// FormatTag formats a version as a git tag (e.g., "v1.2.3").
func FormatTag(v Version) string {
	return v.Tag()
}
