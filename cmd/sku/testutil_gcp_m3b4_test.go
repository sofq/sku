package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededGCPM3b4Catalog struct{ dataDir string }

// testutilSeededGCPCatalogM3b4 creates a SKU_DATA_DIR containing gcp-gcs.db,
// gcp-run.db, and gcp-functions.db, each populated from seed_gcp_m3b4.sql.
func testutilSeededGCPCatalogM3b4(t *testing.T) seededGCPM3b4Catalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_gcp_m3b4.sql"))
	require.NoError(t, err)
	for _, shard := range []string{"gcp-gcs", "gcp-run", "gcp-functions"} {
		require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, shard+".db"), string(ddl)))
	}
	t.Setenv("SKU_DATA_DIR", dir)
	return seededGCPM3b4Catalog{dataDir: dir}
}
