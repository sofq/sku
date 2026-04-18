package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureBlobPrice_HappyPath(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	out, _, code := runAzure(t, "azure", "blob", "price",
		"--tier", "hot",
		"--region", "eastus",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "azure", env["provider"])
	require.Equal(t, "blob", env["service"])
	resource := env["resource"].(map[string]any)
	require.Equal(t, "hot", resource["name"])
	require.Contains(t, out, "0.0184")
}

func TestAzureBlobPrice_RequiresTier(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	_, stderr, code := runAzure(t, "azure", "blob", "price", "--region", "eastus")
	require.NotZero(t, code)
	require.Contains(t, stderr, "tier")
}

func TestAzureBlobPrice_NotFound_ReturnsExit3(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	_, stderr, code := runAzure(t, "azure", "blob", "price",
		"--tier", "cool", // seed has hot + archive only
		"--region", "eastus",
	)
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
}

func TestAzureBlobPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "azure", "blob", "price",
		"--tier", "hot",
		"--region", "eastus",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"azure blob price"`)
	require.Contains(t, out, "azure-blob")
}

func TestAzureBlobList_DropsRegion(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	out, _, code := runAzure(t, "azure", "blob", "list", "--tier", "hot")
	require.Zero(t, code)
	lines := splitLines(out)
	require.NotEmpty(t, lines)
	require.Contains(t, out, "eastus")
}
