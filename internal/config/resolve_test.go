package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/config"
)

func TestResolve_Defaults_WhenAllUnset(t *testing.T) {
	s, err := config.Resolve(config.FlagBag{}, config.File{}, map[string]string{})
	require.NoError(t, err)
	require.Equal(t, "agent", s.Preset)
	require.Equal(t, "json", s.Format)
	require.Equal(t, 14, s.StaleWarningDays)
	require.Equal(t, 0, s.StaleErrorDays)
	require.False(t, s.Pretty)
}

func TestResolve_EnvOverridesProfile(t *testing.T) {
	f := config.File{Profiles: map[string]config.Profile{
		"default": {Preset: "full"},
	}}
	s, _ := config.Resolve(config.FlagBag{}, f, map[string]string{"SKU_PRESET": "price"})
	require.Equal(t, "price", s.Preset)
}

func TestResolve_FlagOverridesEnv(t *testing.T) {
	s, _ := config.Resolve(
		config.FlagBag{Preset: "compare", PresetSet: true},
		config.File{},
		map[string]string{"SKU_PRESET": "price"},
	)
	require.Equal(t, "compare", s.Preset)
}

func TestResolve_UnknownProfile_ReturnsError(t *testing.T) {
	_, err := config.Resolve(
		config.FlagBag{Profile: "bogus", ProfileSet: true},
		config.File{Profiles: map[string]config.Profile{"default": {}}},
		map[string]string{},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bogus")
}

func TestResolve_NoColorStandardEnv(t *testing.T) {
	s, _ := config.Resolve(config.FlagBag{}, config.File{}, map[string]string{"NO_COLOR": "1"})
	require.True(t, s.NoColor)
}
