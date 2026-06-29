package semver

import (
	"testing"
)

// Test ParseVersion with simple version
func TestParseVersion_Simple(t *testing.T) {
	v, err := ParseVersion("1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Major != 1 || v.Minor != 2 || v.Patch != 3 {
		t.Errorf("expected Version{1, 2, 3}, got %v", v)
	}
}

// Test ParseVersion with v prefix
func TestParseVersion_WithVPrefix(t *testing.T) {
	v, err := ParseVersion("v1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Major != 1 || v.Minor != 2 || v.Patch != 3 {
		t.Errorf("expected Version{1, 2, 3}, got %v", v)
	}
}

// Test ParseVersion with invalid input
func TestParseVersion_Invalid(t *testing.T) {
	_, err := ParseVersion("not-a-version")
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

// Test ParseVersion with empty string
func TestParseVersion_Empty(t *testing.T) {
	_, err := ParseVersion("")
	if err == nil {
		t.Error("expected error for empty version")
	}
}

// Test ParseVersion with zero version
func TestParseVersion_ZeroVersion(t *testing.T) {
	v, err := ParseVersion("0.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Major != 0 || v.Minor != 0 || v.Patch != 0 {
		t.Errorf("expected Version{0, 0, 0}, got %v", v)
	}
}

// Test NextVersion with BumpPatch
func TestNextVersion_BumpPatch(t *testing.T) {
	current := Version{1, 2, 3}
	next := NextVersion(current, BumpPatch)
	if next.Major != 1 || next.Minor != 2 || next.Patch != 4 {
		t.Errorf("expected Version{1, 2, 4}, got %v", next)
	}
}

// Test NextVersion with BumpMinor
func TestNextVersion_BumpMinor(t *testing.T) {
	current := Version{1, 2, 3}
	next := NextVersion(current, BumpMinor)
	if next.Major != 1 || next.Minor != 3 || next.Patch != 0 {
		t.Errorf("expected Version{1, 3, 0}, got %v", next)
	}
}

// Test NextVersion with BumpMajor
func TestNextVersion_BumpMajor(t *testing.T) {
	current := Version{1, 2, 3}
	next := NextVersion(current, BumpMajor)
	if next.Major != 2 || next.Minor != 0 || next.Patch != 0 {
		t.Errorf("expected Version{2, 0, 0}, got %v", next)
	}
}

// Test NextVersion with BumpNone
func TestNextVersion_BumpNone(t *testing.T) {
	current := Version{1, 2, 3}
	next := NextVersion(current, BumpNone)
	if next.Major != 1 || next.Minor != 2 || next.Patch != 3 {
		t.Errorf("expected Version{1, 2, 3}, got %v", next)
	}
}

// Test NextVersion with zero version and BumpPatch
func TestNextVersion_ZeroPatch(t *testing.T) {
	current := Version{0, 0, 0}
	next := NextVersion(current, BumpPatch)
	if next.Major != 0 || next.Minor != 0 || next.Patch != 1 {
		t.Errorf("expected Version{0, 0, 1}, got %v", next)
	}
}

// Test NextVersion with zero version and BumpMinor
func TestNextVersion_ZeroMinor(t *testing.T) {
	current := Version{0, 0, 0}
	next := NextVersion(current, BumpMinor)
	if next.Major != 0 || next.Minor != 1 || next.Patch != 0 {
		t.Errorf("expected Version{0, 1, 0}, got %v", next)
	}
}

// Test NextVersion with zero version and BumpMajor
func TestNextVersion_ZeroMajor(t *testing.T) {
	current := Version{0, 0, 0}
	next := NextVersion(current, BumpMajor)
	if next.Major != 1 || next.Minor != 0 || next.Patch != 0 {
		t.Errorf("expected Version{1, 0, 0}, got %v", next)
	}
}

// Test DetectBumpType with feat
func TestDetectBumpType_Feat(t *testing.T) {
	bump := DetectBumpType([]string{"feat"})
	if bump != BumpMinor {
		t.Errorf("expected BumpMinor, got %v", bump)
	}
}

// Test DetectBumpType with fix
func TestDetectBumpType_Fix(t *testing.T) {
	bump := DetectBumpType([]string{"fix"})
	if bump != BumpPatch {
		t.Errorf("expected BumpPatch, got %v", bump)
	}
}

// Test DetectBumpType with breaking-change (hyphenated)
func TestDetectBumpType_BreakingChange(t *testing.T) {
	bump := DetectBumpType([]string{"breaking-change"})
	if bump != BumpMajor {
		t.Errorf("expected BumpMajor, got %v", bump)
	}
}

// Test DetectBumpType with multiple types (feat + fix)
func TestDetectBumpType_FeatAndFix(t *testing.T) {
	bump := DetectBumpType([]string{"feat", "fix"})
	if bump != BumpMinor {
		t.Errorf("expected BumpMinor (highest), got %v", bump)
	}
}

// Test DetectBumpType with multiple types (breaking-change + feat)
func TestDetectBumpType_BreakingAndFeat(t *testing.T) {
	bump := DetectBumpType([]string{"breaking-change", "feat"})
	if bump != BumpMajor {
		t.Errorf("expected BumpMajor (highest), got %v", bump)
	}
}

// Test DetectBumpType with empty list
func TestDetectBumpType_Empty(t *testing.T) {
	bump := DetectBumpType([]string{})
	if bump != BumpNone {
		t.Errorf("expected BumpNone, got %v", bump)
	}
}

// Test DetectBumpType with unknown type
func TestDetectBumpType_Unknown(t *testing.T) {
	bump := DetectBumpType([]string{"unknown"})
	if bump != BumpNone {
		t.Errorf("expected BumpNone, got %v", bump)
	}
}

// Test BootstrapVersion with feat
func TestBootstrapVersion_Feat(t *testing.T) {
	v := BootstrapVersion([]string{"feat"})
	if v.Major != 0 || v.Minor != 1 || v.Patch != 0 {
		t.Errorf("expected Version{0, 1, 0}, got %v", v)
	}
}

// Test BootstrapVersion with fix
func TestBootstrapVersion_Fix(t *testing.T) {
	v := BootstrapVersion([]string{"fix"})
	if v.Major != 0 || v.Minor != 0 || v.Patch != 1 {
		t.Errorf("expected Version{0, 0, 1}, got %v", v)
	}
}

// Test BootstrapVersion with empty list
func TestBootstrapVersion_Empty(t *testing.T) {
	v := BootstrapVersion([]string{})
	if v.Major != 0 || v.Minor != 0 || v.Patch != 0 {
		t.Errorf("expected Version{0, 0, 0}, got %v", v)
	}
}

// Test BootstrapVersion with BREAKING CHANGE
func TestBootstrapVersion_BreakingChange(t *testing.T) {
	v := BootstrapVersion([]string{"breaking-change"})
	if v.Major != 1 || v.Minor != 0 || v.Patch != 0 {
		t.Errorf("expected Version{1, 0, 0}, got %v", v)
	}
}

// Test FormatVersion
func TestFormatVersion(t *testing.T) {
	v := Version{1, 2, 3}
	result := FormatVersion(v)
	if result != "1.2.3" {
		t.Errorf("expected \"1.2.3\", got %q", result)
	}
}

// Test FormatTag
func TestFormatTag(t *testing.T) {
	v := Version{1, 2, 3}
	result := FormatTag(v)
	if result != "v1.2.3" {
		t.Errorf("expected \"v1.2.3\", got %q", result)
	}
}

// Test Version.String()
func TestVersion_String(t *testing.T) {
	v := Version{0, 1, 0}
	result := v.String()
	if result != "0.1.0" {
		t.Errorf("expected \"0.1.0\", got %q", result)
	}
}

// Test Version.Tag()
func TestVersion_Tag(t *testing.T) {
	v := Version{0, 1, 0}
	result := v.Tag()
	if result != "v0.1.0" {
		t.Errorf("expected \"v0.1.0\", got %q", result)
	}
}

// Test table-driven tests for comprehensive coverage
func TestDetectBumpType_Comprehensive(t *testing.T) {
	tests := []struct {
		name      string
		types     []string
		expected  BumpType
	}{
		{"single feat", []string{"feat"}, BumpMinor},
		{"single fix", []string{"fix"}, BumpPatch},
		{"single breaking", []string{"breaking-change"}, BumpMajor},
		{"feat and fix", []string{"feat", "fix"}, BumpMinor},
		{"breaking and feat", []string{"breaking-change", "feat"}, BumpMajor},
		{"breaking and fix", []string{"breaking-change", "fix"}, BumpMajor},
		{"all three", []string{"breaking-change", "feat", "fix"}, BumpMajor},
		{"empty", []string{}, BumpNone},
		{"unknown", []string{"unknown"}, BumpNone},
		{"multiple unknown", []string{"unknown1", "unknown2"}, BumpNone},
		{"mixed known and unknown", []string{"feat", "unknown"}, BumpMinor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBumpType(tt.types)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBootstrapVersion_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		types    []string
		expected Version
	}{
		{"feat", []string{"feat"}, Version{0, 1, 0}},
		{"fix", []string{"fix"}, Version{0, 0, 1}},
		{"breaking", []string{"breaking-change"}, Version{1, 0, 0}},
		{"empty", []string{}, Version{0, 0, 0}},
		{"unknown", []string{"unknown"}, Version{0, 0, 0}},
		{"feat and fix", []string{"feat", "fix"}, Version{0, 1, 0}},
		{"breaking and feat", []string{"breaking-change", "feat"}, Version{1, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BootstrapVersion(tt.types)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// --- FormatTagWithPrefix tests ---

func TestFormatTagWithPrefix_V(t *testing.T) {
	if got := FormatTagWithPrefix(Version{1, 2, 3}, "v"); got != "v1.2.3" {
		t.Errorf("expected %q, got %q", "v1.2.3", got)
	}
}

func TestFormatTagWithPrefix_Empty(t *testing.T) {
	if got := FormatTagWithPrefix(Version{1, 2, 3}, ""); got != "1.2.3" {
		t.Errorf("expected %q, got %q", "1.2.3", got)
	}
}

func TestFormatTagWithPrefix_Custom(t *testing.T) {
	if got := FormatTagWithPrefix(Version{1, 2, 3}, "release-"); got != "release-1.2.3" {
		t.Errorf("expected %q, got %q", "release-1.2.3", got)
	}
}

func TestFormatTagWithPrefix_MatchesTagMethod(t *testing.T) {
	versions := []Version{{1, 2, 3}, {0, 0, 0}, {0, 1, 0}, {10, 20, 30}}
	for _, v := range versions {
		if got := FormatTagWithPrefix(v, "v"); got != v.Tag() {
			t.Errorf("FormatTagWithPrefix(v, \"v\") = %q, v.Tag() = %q", got, v.Tag())
		}
	}
}

// --- ParseVersionFromTag tests ---

func TestParseVersionFromTag_OK(t *testing.T) {
	v, err := ParseVersionFromTag("v1.2.3", "v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != (Version{1, 2, 3}) {
		t.Errorf("expected Version{1,2,3}, got %v", v)
	}
}

func TestParseVersionFromTag_CustomPrefix(t *testing.T) {
	v, err := ParseVersionFromTag("release-2.0.0", "release-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != (Version{2, 0, 0}) {
		t.Errorf("expected Version{2,0,0}, got %v", v)
	}
}

func TestParseVersionFromTag_EmptyPrefix(t *testing.T) {
	v, err := ParseVersionFromTag("1.2.3", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != (Version{1, 2, 3}) {
		t.Errorf("expected Version{1,2,3}, got %v", v)
	}
}

func TestParseVersionFromTag_WrongPrefix(t *testing.T) {
	_, err := ParseVersionFromTag("v1.0.0", "release-")
	if err == nil {
		t.Error("expected error when tag does not start with prefix, got nil")
	}
}

// --- DetectBumpType variadic rules tests ---

func TestDetectBumpType_WithRules_Major(t *testing.T) {
	rules := map[string]string{"breaking-change": "major"}
	bump := DetectBumpType([]string{"breaking-change"}, rules)
	if bump != BumpMajor {
		t.Errorf("expected BumpMajor, got %v", bump)
	}
}

func TestDetectBumpType_WithRules_Override(t *testing.T) {
	rules := map[string]string{"docs": "minor"}
	bump := DetectBumpType([]string{"docs"}, rules)
	if bump != BumpMinor {
		t.Errorf("expected BumpMinor, got %v", bump)
	}
}

func TestDetectBumpType_WithRules_None(t *testing.T) {
	// "none" is not a recognised level — treated as unknown → BumpNone
	rules := map[string]string{"fix": "none"}
	bump := DetectBumpType([]string{"fix"}, rules)
	if bump != BumpNone {
		t.Errorf("expected BumpNone, got %v", bump)
	}
}

func TestDetectBumpType_WithRules_HighestWins(t *testing.T) {
	rules := map[string]string{"feat": "minor", "fix": "major"}
	bump := DetectBumpType([]string{"feat", "fix"}, rules)
	if bump != BumpMajor {
		t.Errorf("expected BumpMajor, got %v", bump)
	}
}

// --- Built-in sentinel migration tests ---

func TestDetectBumpType_BuiltinBreakingChange(t *testing.T) {
	bump := DetectBumpType([]string{"breaking-change"})
	if bump != BumpMajor {
		t.Errorf("expected BumpMajor, got %v", bump)
	}
}

func TestDetectBumpType_BuiltinBREAKING_CHANGE_Removed(t *testing.T) {
	// Old "BREAKING CHANGE" (with space) sentinel is no longer in the built-in switch.
	bump := DetectBumpType([]string{"BREAKING CHANGE"})
	if bump != BumpNone {
		t.Errorf("expected BumpNone (old sentinel removed), got %v", bump)
	}
}

func TestDetectBumpType_NoRules_Feat(t *testing.T) {
	bump := DetectBumpType([]string{"feat"})
	if bump != BumpMinor {
		t.Errorf("expected BumpMinor, got %v", bump)
	}
}

func TestDetectBumpType_NoRules_Fix(t *testing.T) {
	bump := DetectBumpType([]string{"fix"})
	if bump != BumpPatch {
		t.Errorf("expected BumpPatch, got %v", bump)
	}
}

func TestNextVersion_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		current  Version
		bump     BumpType
		expected Version
	}{
		{"patch bump", Version{1, 2, 3}, BumpPatch, Version{1, 2, 4}},
		{"minor bump", Version{1, 2, 3}, BumpMinor, Version{1, 3, 0}},
		{"major bump", Version{1, 2, 3}, BumpMajor, Version{2, 0, 0}},
		{"no bump", Version{1, 2, 3}, BumpNone, Version{1, 2, 3}},
		{"zero patch", Version{0, 0, 0}, BumpPatch, Version{0, 0, 1}},
		{"zero minor", Version{0, 0, 0}, BumpMinor, Version{0, 1, 0}},
		{"zero major", Version{0, 0, 0}, BumpMajor, Version{1, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NextVersion(tt.current, tt.bump)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseVersion_Comprehensive(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		want    Version
	}{
		{"simple", "1.2.3", false, Version{1, 2, 3}},
		{"with v prefix", "v1.2.3", false, Version{1, 2, 3}},
		{"zero version", "0.0.0", false, Version{0, 0, 0}},
		{"large numbers", "10.20.30", false, Version{10, 20, 30}},
		{"invalid", "not-a-version", true, Version{}},
		{"empty", "", true, Version{}},
		{"two parts (1.2 -> 1.2.0)", "1.2", false, Version{1, 2, 0}},
		{"extra parts", "1.2.3.4", true, Version{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && result != tt.want {
				t.Errorf("expected %v, got %v", tt.want, result)
			}
		})
	}
}

func TestVersionFormatting(t *testing.T) {
	tests := []struct {
		name       string
		v          Version
		wantString string
		wantTag    string
	}{
		{"simple", Version{1, 2, 3}, "1.2.3", "v1.2.3"},
		{"zero", Version{0, 0, 0}, "0.0.0", "v0.0.0"},
		{"bootstrap minor", Version{0, 1, 0}, "0.1.0", "v0.1.0"},
		{"bootstrap patch", Version{0, 0, 1}, "0.0.1", "v0.0.1"},
		{"major", Version{1, 0, 0}, "1.0.0", "v1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.String(); got != tt.wantString {
				t.Errorf("String() expected %q, got %q", tt.wantString, got)
			}
			if got := tt.v.Tag(); got != tt.wantTag {
				t.Errorf("Tag() expected %q, got %q", tt.wantTag, got)
			}
			if got := FormatVersion(tt.v); got != tt.wantString {
				t.Errorf("FormatVersion() expected %q, got %q", tt.wantString, got)
			}
			if got := FormatTag(tt.v); got != tt.wantTag {
				t.Errorf("FormatTag() expected %q, got %q", tt.wantTag, got)
			}
		})
	}
}
