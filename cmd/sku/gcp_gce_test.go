package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPGCEPrice_HappyPath(t *testing.T) {
	testutilSeededGCPCatalogM3b3(t)

	out, _, code := runAzure(t, "gcp", "gce", "price",
		"--machine-type", "n1-standard-2",
		"--region", "us-east1",
		"--os", "linux",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "gcp", env["provider"])
	require.Equal(t, "gce", env["service"])
	resource := env["resource"].(map[string]any)
	require.Equal(t, "n1-standard-2", resource["name"])
	require.Contains(t, out, "0.095")
}

func TestGCPGCEPrice_RequiresMachineType(t *testing.T) {
	testutilSeededGCPCatalogM3b3(t)

	_, stderr, code := runAzure(t, "gcp", "gce", "price", "--region", "us-east1")
	require.NotZero(t, code)
	require.Contains(t, stderr, "machine-type")
}

func TestGCPGCEPrice_NotFound_ReturnsExit3(t *testing.T) {
	testutilSeededGCPCatalogM3b3(t)

	_, stderr, code := runAzure(t, "gcp", "gce", "price",
		"--machine-type", "n99-standard-99",
		"--region", "us-east1",
		"--os", "linux",
	)
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
}

func TestGCPGCEPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "gcp", "gce", "price",
		"--machine-type", "n1-standard-2",
		"--region", "us-east1",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"gcp gce price"`)
	require.Contains(t, out, "gcp-gce")
}

func TestGCPGCEList_HappyPath(t *testing.T) {
	testutilSeededGCPCatalogM3b3(t)

	out, _, code := runAzure(t, "gcp", "gce", "list",
		"--machine-type", "n1-standard-2",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.GreaterOrEqual(t, len(lines), 1)
}
