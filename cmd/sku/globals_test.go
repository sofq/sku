package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
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
