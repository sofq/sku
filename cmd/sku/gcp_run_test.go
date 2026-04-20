package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPRunPrice_HappyPath(t *testing.T) {
	testutilSeededGCPCatalogM3b4(t)

	out, _, code := runAzure(t, "gcp", "run", "price",
		"--architecture", "x86_64",
		"--region", "us-east1",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "gcp", env["provider"])
	require.Equal(t, "run", env["service"])
}

func TestGCPRunList_RegionOptional(t *testing.T) {
	testutilSeededGCPCatalogM3b4(t)

	out, _, code := runAzure(t, "gcp", "run", "list", "--architecture", "x86_64")
	require.Zero(t, code)
	require.Contains(t, out, "x86_64")
}

func TestGCPRunPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "gcp", "run", "price",
		"--architecture", "x86_64",
		"--region", "us-east1",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"command":"gcp run price"`)
	require.Contains(t, out, "gcp-run")
}
