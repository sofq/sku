package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededGCPMemorystoreCatalog struct{ dataDir string }

// testutilSeededGCPMemorystoreCatalog creates a SKU_DATA_DIR containing gcp-memorystore.db
// populated from seed_gcp_m_gamma_1_memorystore.sql.
func testutilSeededGCPMemorystoreCatalog(t *testing.T) seededGCPMemorystoreCatalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_gcp_m_gamma_1_memorystore.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "gcp-memorystore.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
	return seededGCPMemorystoreCatalog{dataDir: dir}
}
