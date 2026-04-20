package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureDisksPrice_HappyPath(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	out, _, code := runAzure(t, "azure", "disks", "price",
		"--disk-type", "standard-ssd",
		"--region", "eastus",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "azure", env["provider"])
	require.Equal(t, "disks", env["service"])
	resource := env["resource"].(map[string]any)
	require.Equal(t, "standard-ssd", resource["name"])
	require.Contains(t, out, "4.8")
}

func TestAzureDisksPrice_RequiresDiskType(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	_, stderr, code := runAzure(t, "azure", "disks", "price", "--region", "eastus")
	require.NotZero(t, code)
	require.Contains(t, stderr, "disk-type")
}

func TestAzureDisksPrice_NotFound_ReturnsExit3(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	_, stderr, code := runAzure(t, "azure", "disks", "price",
		"--disk-type", "standard-hdd", // seed has ssd + premium-ssd only
		"--region", "eastus",
	)
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
}

func TestAzureDisksPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "azure", "disks", "price",
		"--disk-type", "premium-ssd",
		"--region", "eastus",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"azure disks price"`)
	require.Contains(t, out, "azure-disks")
}
