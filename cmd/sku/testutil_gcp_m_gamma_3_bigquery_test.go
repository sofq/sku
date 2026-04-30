package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededGCPBigQueryCatalog struct{ dataDir string }

func testutilSeededGCPBigQueryCatalog(t *testing.T) seededGCPBigQueryCatalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_gcp_bigquery.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "gcp-bigquery.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
	return seededGCPBigQueryCatalog{dataDir: dir}
}
