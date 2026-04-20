package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededAWSM3a3Catalog struct{ dataDir string }

// testutilSeededAWSCatalogM3a3 creates a SKU_DATA_DIR containing
// aws-dynamodb.db and aws-cloudfront.db, both populated from
// internal/catalog/testdata/seed_aws_m3a3.sql. The seed file is fully
// self-contained (DDL + rows), so BuildFromSQL is a one-shot call per shard.
func testutilSeededAWSCatalogM3a3(t *testing.T) seededAWSM3a3Catalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_aws_m3a3.sql"))
	require.NoError(t, err)
	for _, shard := range []string{"aws-dynamodb", "aws-cloudfront"} {
		require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, shard+".db"), string(ddl)))
	}
	t.Setenv("SKU_DATA_DIR", dir)
	return seededAWSM3a3Catalog{dataDir: dir}
}
