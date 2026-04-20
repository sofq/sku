package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPFunctionsPrice_HappyPath(t *testing.T) {
	testutilSeededGCPCatalogM3b4(t)

	out, _, code := runAzure(t, "gcp", "functions", "price",
		"--architecture", "x86_64",
		"--region", "europe-west1",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "gcp", env["provider"])
	require.Equal(t, "functions", env["service"])
}

func TestGCPFunctionsList_RegionOptional(t *testing.T) {
	testutilSeededGCPCatalogM3b4(t)

	out, _, code := runAzure(t, "gcp", "functions", "list", "--architecture", "x86_64")
	require.Zero(t, code)
	require.Contains(t, out, "x86_64")
}

func TestGCPFunctionsPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "gcp", "functions", "price",
		"--architecture", "x86_64",
		"--region", "us-east1",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"command":"gcp functions price"`)
	require.Contains(t, out, "gcp-functions")
}
