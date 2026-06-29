package conventional

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LintRules controls which lint rules are enforced by ValidateAll.
type LintRules struct {
	// CapitalFirstLetter fails commits whose description starts with an
	// uppercase letter. Default: true.
	CapitalFirstLetter bool `yaml:"capital-first-letter"`
	// RequireScope fails commits that have no scope. Default: false.
	RequireScope bool `yaml:"require-scope"`
}

// LintConfig is the [lint] section of .semrelrc.yml.
type LintConfig struct {
	Rules LintRules `yaml:"rules"`
}

// Config is the root structure of .semrelrc.yml.
type Config struct {
	Lint LintConfig `yaml:"lint"`
}

// DefaultConfig returns the hardcoded defaults (current behaviour — no file needed).
func DefaultConfig() Config {
	return Config{
		Lint: LintConfig{
			Rules: LintRules{
				CapitalFirstLetter: true,
				RequireScope:       false,
			},
		},
	}
}

// LintOptions is derived from Config.Lint.Rules and passed to ValidateAll.
type LintOptions struct {
	CapitalFirstLetter bool
	RequireScope       bool
}

// DefaultLintOptions returns options matching the hardcoded defaults.
func DefaultLintOptions() LintOptions {
	d := DefaultConfig()
	return LintOptions{
		CapitalFirstLetter: d.Lint.Rules.CapitalFirstLetter,
		RequireScope:       d.Lint.Rules.RequireScope,
	}
}

// LoadConfig reads .semrelrc.yml from path.
//   - Returns (nil, nil) if the file does not exist — caller uses DefaultConfig().
//   - Returns (nil, err) if the file exists but contains malformed YAML.
//   - Returns (*Config, nil) on success.
//
// IMPORTANT: YAML is decoded onto a pre-initialised DefaultConfig() struct so
// that fields absent from the file retain their default values (not zero values).
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read .semrelrc.yml: %w", err)
	}
	cfg := DefaultConfig() // decode onto defaults, not zero-value
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("malformed .semrelrc.yml: %w", err)
	}
	return &cfg, nil
}
