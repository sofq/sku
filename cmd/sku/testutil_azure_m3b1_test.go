package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededAzureM3b1Catalog struct{ dataDir string }

// testutilSeededAzureCatalogM3b1 creates a SKU_DATA_DIR containing
// azure-vm.db and azure-sql.db, both populated from
// internal/catalog/testdata/seed_azure.sql. The seed file is fully
// self-contained (DDL + rows), so BuildFromSQL is a one-shot call per shard.
func testutilSeededAzureCatalogM3b1(t *testing.T) seededAzureM3b1Catalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_azure.sql"))
	require.NoError(t, err)
	for _, shard := range []string{"azure-vm", "azure-sql"} {
		require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, shard+".db"), string(ddl)))
	}
	t.Setenv("SKU_DATA_DIR", dir)
	return seededAzureM3b1Catalog{dataDir: dir}
}
