package conventional

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- LoadConfig tests ---

func TestLoadConfig_FileAbsent(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), ".semrelrc.yml"))
	assert.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfig_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".semrelrc.yml")
	require.NoError(t, os.WriteFile(path, []byte(":\tinvalid: yaml:\t["), 0o600))

	cfg, err := LoadConfig(path)
	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "malformed")
}

func TestLoadConfig_DisableCapitalFirstLetter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".semrelrc.yml")
	content := "lint:\n  rules:\n    capital-first-letter: false\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.False(t, cfg.Lint.Rules.CapitalFirstLetter)
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	// A file that sets only capital-first-letter: false should leave require-scope at its default (false).
	dir := t.TempDir()
	path := filepath.Join(dir, ".semrelrc.yml")
	content := "lint:\n  rules:\n    capital-first-letter: false\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.False(t, cfg.Lint.Rules.CapitalFirstLetter, "explicitly set field should be false")
	assert.False(t, cfg.Lint.Rules.RequireScope, "unset field should retain default value (false)")
}

// --- DefaultConfig new-field tests ---

func TestDefaultConfig_TagPrefixIsV(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "v", cfg.TagPrefix, "TagPrefix MUST default to 'v' — empty default silently strips the prefix from all future tags")
}

func TestDefaultConfig_NewFieldDefaults(t *testing.T) {
	cfg := DefaultConfig()

	// BumpRules: exactly 3 entries
	require.Len(t, cfg.BumpRules, 3)
	assert.Equal(t, BumpLevelMajor, cfg.BumpRules["breaking-change"])
	assert.Equal(t, BumpLevelMinor, cfg.BumpRules["feat"])
	assert.Equal(t, BumpLevelPatch, cfg.BumpRules["fix"])

	// ReleaseBranches
	assert.Equal(t, []string{"main", "master"}, cfg.ReleaseBranches)

	// InitialVersion
	assert.Equal(t, "0.0.0", cfg.InitialVersion)
}

// --- LoadConfig new-field tests ---

func TestLoadConfig_AllNewFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".semrelrc.yml")
	content := `
bump-rules:
  breaking-change: major
  feat: minor
  fix: patch
  chore: none
release-branches:
  - main
  - release/*
tag-prefix: "ver-"
commit-types:
  extra-types:
    - custom
  allowed-types:
    - feat
    - fix
    - custom
initial-version: "1.0.0"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, BumpLevelMajor, cfg.BumpRules["breaking-change"])
	assert.Equal(t, BumpLevelMinor, cfg.BumpRules["feat"])
	assert.Equal(t, BumpLevelPatch, cfg.BumpRules["fix"])
	assert.Equal(t, BumpLevelNone, cfg.BumpRules["chore"])
	assert.Equal(t, []string{"main", "release/*"}, cfg.ReleaseBranches)
	assert.Equal(t, "ver-", cfg.TagPrefix)
	assert.Equal(t, []string{"custom"}, cfg.CommitTypes.ExtraTypes)
	assert.Equal(t, []CommitType{"feat", "fix", "custom"}, cfg.CommitTypes.AllowedTypes)
	assert.Equal(t, "1.0.0", cfg.InitialVersion)
}

func TestLoadConfig_BumpRulesNilGuard_BareNull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".semrelrc.yml")
	content := "bump-rules:\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Len(t, cfg.BumpRules, 3, "bare null bump-rules should restore the 3 defaults")
	assert.Equal(t, BumpLevelMajor, cfg.BumpRules["breaking-change"])
	assert.Equal(t, BumpLevelMinor, cfg.BumpRules["feat"])
	assert.Equal(t, BumpLevelPatch, cfg.BumpRules["fix"])
}

func TestLoadConfig_BumpRulesNilGuard_EmptyMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".semrelrc.yml")
	content := "bump-rules: {}\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Len(t, cfg.BumpRules, 3, "empty map bump-rules should restore the 3 defaults")
}

func TestLoadConfig_BumpRulesInvalidLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".semrelrc.yml")
	content := "bump-rules:\n  feat: superminor\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := LoadConfig(path)
	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bump level")
}

func TestLoadConfig_TagPrefixDefault_Absent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".semrelrc.yml")
	content := "lint:\n  rules:\n    require-scope: true\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "v", cfg.TagPrefix, "absent tag-prefix should retain default 'v'")
}

func TestLoadConfig_TagPrefixEmpty_Explicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".semrelrc.yml")
	content := "tag-prefix: \"\"\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "", cfg.TagPrefix, "explicit empty tag-prefix should be respected as user opt-in")
}

// --- ValidateAll option tests ---

func TestValidateAll_CapitalFirstLetter_Disabled(t *testing.T) {
	commits := []RawCommit{{SHA: "abc1234567890", Message: "feat: Uppercase description"}}
	opts := LintOptions{CapitalFirstLetter: false, RequireScope: false}
	violations := ValidateAll(commits, opts)
	assert.Empty(t, violations)
}

func TestValidateAll_CapitalFirstLetter_Enabled(t *testing.T) {
	commits := []RawCommit{{SHA: "abc1234567890", Message: "feat: Uppercase description"}}
	opts := LintOptions{CapitalFirstLetter: true, RequireScope: false}
	violations := ValidateAll(commits, opts)
	require.Len(t, violations, 1)
	assert.Equal(t, "capital-first-letter", violations[0].Rule)
}

func TestValidateAll_RequireScope_Enabled(t *testing.T) {
	commits := []RawCommit{{SHA: "abc1234567890", Message: "feat: no scope here"}}
	opts := LintOptions{CapitalFirstLetter: false, RequireScope: true}
	violations := ValidateAll(commits, opts)
	require.Len(t, violations, 1)
	assert.Equal(t, "missing-scope", violations[0].Rule)
}

func TestValidateAll_RequireScope_Disabled(t *testing.T) {
	commits := []RawCommit{{SHA: "abc1234567890", Message: "feat: no scope here"}}
	opts := LintOptions{CapitalFirstLetter: false, RequireScope: false}
	violations := ValidateAll(commits, opts)
	assert.Empty(t, violations)
}

func TestValidateAll_WithNoOpts_UsesDefaults(t *testing.T) {
	// ValidateAll(commits) with no opts should behave identically to ValidateAll(commits, DefaultLintOptions()).
	commits := []RawCommit{
		{SHA: "abc1234567890", Message: "feat: Uppercase"},
		{SHA: "def1234567890", Message: "fix: lowercase no scope"},
	}
	withoutOpts := ValidateAll(commits)
	withDefaults := ValidateAll(commits, DefaultLintOptions())
	assert.Equal(t, withDefaults, withoutOpts)
}
