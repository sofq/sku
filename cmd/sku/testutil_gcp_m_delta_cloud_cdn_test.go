package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededGCPCloudCDNCatalog struct{ dataDir string }

// testutilSeededGCPCloudCDNCatalog creates a SKU_DATA_DIR containing gcp-cloud-cdn.db
// populated from seed_gcp_cloud_cdn.sql.
func testutilSeededGCPCloudCDNCatalog(t *testing.T) seededGCPCloudCDNCatalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_gcp_cloud_cdn.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "gcp-cloud-cdn.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
	return seededGCPCloudCDNCatalog{dataDir: dir}
}
