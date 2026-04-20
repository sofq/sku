package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SaveProfile merges p into the file at path under name, creating the file
// (and parent dirs) when absent. Merge semantics: p replaces the named
// profile wholesale; other profiles are untouched.
func SaveProfile(path, name string, p Profile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := Load(path)
	if err != nil {
		return err
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	f.Profiles[name] = p
	b, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
