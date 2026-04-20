package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureSQLPrice_HappyPath(t *testing.T) {
	testutilSeededAzureCatalogM3b1(t)

	out, _, code := runAzure(t, "azure", "sql", "price",
		"--sku-name", "GP_Gen5_2",
		"--region", "eastus",
		"--deployment-option", "single-az",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "azure", env["provider"])
	require.Equal(t, "sql", env["service"])
	resource := env["resource"].(map[string]any)
	require.Equal(t, "GP_Gen5_2", resource["name"])
	require.Contains(t, out, "0.252")
}

func TestAzureSQLPrice_ManagedInstance(t *testing.T) {
	testutilSeededAzureCatalogM3b1(t)

	out, _, code := runAzure(t, "azure", "sql", "price",
		"--sku-name", "BC_Gen5_2",
		"--region", "eastus",
		"--deployment-option", "managed-instance",
	)
	require.Zero(t, code)
	require.Contains(t, out, "1.058")
}

func TestAzureSQLPrice_RequiresSkuName(t *testing.T) {
	testutilSeededAzureCatalogM3b1(t)

	_, stderr, code := runAzure(t, "azure", "sql", "price", "--region", "eastus")
	require.NotZero(t, code)
	require.Contains(t, stderr, "sku-name")
}

func TestAzureSQLPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "azure", "sql", "price",
		"--sku-name", "GP_Gen5_2",
		"--region", "eastus",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"azure sql price"`)
	require.Contains(t, out, "azure-sql")
}
