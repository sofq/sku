package sku

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gopkg.in/yaml.v3"
)

func TestConfigure_FlaggedMode_WritesProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SKU_CONFIG_DIR", dir)

	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"configure", "--profile", "ci",
		"--preset", "price",
		"--stale-warning-days", "3",
		"--stale-error-days", "7",
	})
	require.NoError(t, root.Execute())

	b, err := os.ReadFile(filepath.Join(dir, "config.yaml")) //nolint:gosec // test reads a temp dir it controls
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(b, &doc))
	profs := doc["profiles"].(map[string]any)
	ci := profs["ci"].(map[string]any)
	require.Equal(t, "price", ci["preset"])
	require.Equal(t, 3, ci["stale_warning_days"])
	require.Equal(t, 7, ci["stale_error_days"])
}
