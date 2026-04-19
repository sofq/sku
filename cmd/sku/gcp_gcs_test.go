package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPGCSPrice_HappyPath(t *testing.T) {
	testutilSeededGCPCatalogM3b4(t)

	out, _, code := runAzure(t, "gcp", "gcs", "price",
		"--storage-class", "standard",
		"--region", "us-east1",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "gcp", env["provider"])
	require.Equal(t, "gcs", env["service"])
	require.Contains(t, out, "\"name\":\"standard\"")
}

func TestGCPGCSList_RegionOptional(t *testing.T) {
	testutilSeededGCPCatalogM3b4(t)

	out, _, code := runAzure(t, "gcp", "gcs", "list", "--storage-class", "standard")
	require.Zero(t, code)
	require.Contains(t, out, "standard")
}

func TestGCPGCSPrice_MissingStorageClassValidationError(t *testing.T) {
	testutilSeededGCPCatalogM3b4(t)

	_, stderr, code := runAzure(t, "gcp", "gcs", "price", "--region", "us-east1")
	require.NotZero(t, code)
	require.Contains(t, stderr, "storage-class")
}

func TestGCPGCSPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "gcp", "gcs", "price",
		"--storage-class", "standard",
		"--region", "us-east1",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"gcp gcs price"`)
	require.Contains(t, out, "gcp-gcs")
}
