package sku

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	skuerrors "github.com/sofq/sku/internal/errors"
)

func TestRoot_GlobalFlagsRegistered(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{
		"profile", "preset", "jq", "fields", "include-raw", "include-aggregated",
		"pretty", "stale-ok", "auto-fetch", "dry-run", "verbose", "no-color",
		"json", "yaml", "toml",
	} {
		require.NotNil(t, root.PersistentFlags().Lookup(name), "missing global flag --%s", name)
	}
}

func TestRoot_PresetEnvPropagates(t *testing.T) {
	t.Setenv("SKU_PRESET", "price")
	root := newRootCmd()
	root.SetArgs([]string{"version"})
	var out bytes.Buffer
	root.SetOut(&out)
	require.NoError(t, root.Execute())
	// version command doesn't use preset, but Settings must resolve without error.
}

func TestRoot_InvalidPresetRejected(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"--preset", "bogus", "version"})
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	err := root.Execute()
	require.Error(t, err)

	var e *skuerrors.E
	require.True(t, errors.As(err, &e), "expected *skuerrors.E, got %T: %v", err, err)
	require.Equal(t, skuerrors.CodeValidation, e.Code)
	require.Equal(t, 4, e.Code.ExitCode())
	require.Equal(t, "bogus", e.Details["value"])
}

func TestRoot_InvalidPresetFromEnvRejected(t *testing.T) {
	t.Setenv("SKU_PRESET", "nope")
	root := newRootCmd()
	root.SetArgs([]string{"version"})
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	err := root.Execute()
	require.Error(t, err)

	var e *skuerrors.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, skuerrors.CodeValidation, e.Code)
}
