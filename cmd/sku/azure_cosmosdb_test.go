package sku

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureCosmosDBPriceCmd_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "azure", "cosmosdb", "price",
		"--capacity-mode", "provisioned",
		"--region", "eastus",
		"--api", "sql",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"azure cosmosdb price"`)
	require.Contains(t, out, "azure-cosmosdb")
}
