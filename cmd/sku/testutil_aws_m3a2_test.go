package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

type seededAWSM3a2Catalog struct{ dataDir string }

// testutilSeededAWSCatalogM3a2 creates a SKU_DATA_DIR containing
// aws-s3.db, aws-lambda.db, aws-ebs.db all populated from
// internal/catalog/testdata/seed_aws_m3a2.sql.
func testutilSeededAWSCatalogM3a2(t *testing.T) seededAWSM3a2Catalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_aws_m3a2.sql"))
	require.NoError(t, err)
	for _, shard := range []string{"aws-s3", "aws-lambda", "aws-ebs"} {
		require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, shard+".db"), string(ddl)))
	}
	t.Setenv("SKU_DATA_DIR", dir)
	return seededAWSM3a2Catalog{dataDir: dir}
}
