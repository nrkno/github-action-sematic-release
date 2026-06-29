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
