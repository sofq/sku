package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededAzureM3b2Catalog struct{ dataDir string }

// testutilSeededAzureCatalogM3b2 creates a SKU_DATA_DIR containing
// azure-blob.db, azure-functions.db, azure-disks.db — all populated from
// internal/catalog/testdata/seed_azure_m3b2.sql.
func testutilSeededAzureCatalogM3b2(t *testing.T) seededAzureM3b2Catalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_azure_m3b2.sql"))
	require.NoError(t, err)
	for _, shard := range []string{"azure-blob", "azure-functions", "azure-disks"} {
		require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, shard+".db"), string(ddl)))
	}
	t.Setenv("SKU_DATA_DIR", dir)
	return seededAzureM3b2Catalog{dataDir: dir}
}
