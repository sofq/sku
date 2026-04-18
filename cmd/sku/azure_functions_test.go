package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureFunctionsPrice_HappyPath(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	out, _, code := runAzure(t, "azure", "functions", "price",
		"--architecture", "x86_64",
		"--region", "eastus",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "azure", env["provider"])
	require.Equal(t, "functions", env["service"])
	resource := env["resource"].(map[string]any)
	require.Equal(t, "x86_64", resource["name"])
	require.Contains(t, out, "0.000016")
}

func TestAzureFunctionsPrice_RequiresArch(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	_, stderr, code := runAzure(t, "azure", "functions", "price",
		"--architecture", "",
		"--region", "eastus",
	)
	require.NotZero(t, code)
	require.Contains(t, stderr, "architecture")
}

func TestAzureFunctionsPrice_NotFound_ReturnsExit3(t *testing.T) {
	testutilSeededAzureCatalogM3b2(t)

	_, stderr, code := runAzure(t, "azure", "functions", "price",
		"--architecture", "arm64", // seed has x86_64 only
		"--region", "eastus",
	)
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
}

func TestAzureFunctionsPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "azure", "functions", "price",
		"--architecture", "x86_64",
		"--region", "eastus",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"azure functions price"`)
	require.Contains(t, out, "azure-functions")
}
