package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededGCPM3b3Catalog struct{ dataDir string }

// testutilSeededGCPCatalogM3b3 creates a SKU_DATA_DIR containing gcp-gce.db
// and gcp-cloud-sql.db, both populated from seed_gcp.sql. The seed file is
// fully self-contained (DDL + rows), so BuildFromSQL is a one-shot call
// per shard.
func testutilSeededGCPCatalogM3b3(t *testing.T) seededGCPM3b3Catalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_gcp.sql"))
	require.NoError(t, err)
	for _, shard := range []string{"gcp-gce", "gcp-cloud-sql"} {
		require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, shard+".db"), string(ddl)))
	}
	t.Setenv("SKU_DATA_DIR", dir)
	return seededGCPM3b3Catalog{dataDir: dir}
}
