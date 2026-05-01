package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededAWSSNSCatalog struct{ dataDir string }

// testutilSeededAWSSNSCatalog creates a SKU_DATA_DIR containing aws-sns.db,
// populated from internal/catalog/testdata/seed_messaging_topic.sql.
func testutilSeededAWSSNSCatalog(t *testing.T) seededAWSSNSCatalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_messaging_topic.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "aws-sns.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
	return seededAWSSNSCatalog{dataDir: dir}
}
