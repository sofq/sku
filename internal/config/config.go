// Package config parses ~/.config/sku/config.yaml (or platform equivalent)
// and exposes the merged Profile struct. Precedence (CLI > env > profile >
// default) is applied in Resolve, not here.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// File is the on-disk shape of sku's config.yaml.
type File struct {
	Profiles map[string]Profile `yaml:"profiles"`
}

// Profile captures the subset of Settings that can be persisted to the
// config file. Pointer fields distinguish "unset" from the typed zero
// value (e.g. StaleErrorDays=0 means "disabled", not "unset").
type Profile struct {
	Preset           string   `yaml:"preset,omitempty"`
	Channel          string   `yaml:"channel,omitempty"`
	DefaultRegions   []string `yaml:"default_regions,omitempty"`
	StaleWarningDays *int     `yaml:"stale_warning_days,omitempty"`
	StaleErrorDays   *int     `yaml:"stale_error_days,omitempty"`
	AutoFetch        *bool    `yaml:"auto_fetch,omitempty"`
	IncludeRaw       *bool    `yaml:"include_raw,omitempty"`
}

// Dir returns the platform-default config directory (spec §4 Environment
// variables), honoring SKU_CONFIG_DIR.
func Dir() string {
	if v := os.Getenv("SKU_CONFIG_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "sku")
	case "windows":
		if v := os.Getenv("APPDATA"); v != "" {
			return filepath.Join(v, "sku")
		}
		return filepath.Join(home, "AppData", "Roaming", "sku")
	default:
		if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
			return filepath.Join(v, "sku")
		}
		return filepath.Join(home, ".config", "sku")
	}
}

// Path returns the canonical config file path under Dir().
func Path() string { return filepath.Join(Dir(), "config.yaml") }

// Load reads the file at path and parses it. Returns an empty File (no
// error) when the file does not exist, so callers can treat "no config"
// as "defaults only".
func Load(path string) (File, error) {
	b, err := os.ReadFile(path) //nolint:gosec // operator-provided path
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, nil
		}
		return File{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return f, nil
}
