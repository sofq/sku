package config_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/config"
)

func TestLoad_ParsesProfilesYAML(t *testing.T) {
	f, err := config.Load(filepath.Join("testdata", "profiles.yaml"))
	require.NoError(t, err)

	def, ok := f.Profiles["default"]
	require.True(t, ok)
	require.Equal(t, "agent", def.Preset)
	require.NotNil(t, def.StaleWarningDays)
	require.Equal(t, 14, *def.StaleWarningDays)
	require.NotNil(t, def.AutoFetch)
	require.False(t, *def.AutoFetch)

	ci := f.Profiles["ci"]
	require.NotNil(t, ci.StaleErrorDays)
	require.Equal(t, 7, *ci.StaleErrorDays)
}

func TestLoad_MissingFile_ReturnsEmptyFile(t *testing.T) {
	f, err := config.Load(filepath.Join("testdata", "nonexistent.yaml"))
	require.NoError(t, err)
	require.Empty(t, f.Profiles)
}

func TestConfigDir_HonorsEnvOverride(t *testing.T) {
	t.Setenv("SKU_CONFIG_DIR", "/tmp/custom/sku")
	require.Equal(t, "/tmp/custom/sku", config.Dir())
}
