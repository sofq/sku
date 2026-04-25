package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededGCPSpannerCatalog struct{ dataDir string }

// testutilSeededGCPSpannerCatalog creates a SKU_DATA_DIR containing gcp-spanner.db
// populated from seed_gcp_m_gamma_1_spanner.sql.
func testutilSeededGCPSpannerCatalog(t *testing.T) seededGCPSpannerCatalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_gcp_m_gamma_1_spanner.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "gcp-spanner.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
	return seededGCPSpannerCatalog{dataDir: dir}
}
