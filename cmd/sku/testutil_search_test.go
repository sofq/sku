package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

// testutilSeededSearchCatalog builds the seed_search.sql fixture into a
// temporary SKU_DATA_DIR and returns the shard path. Every sku search
// Cobra test goes through this helper so the layout stays consistent.
func testutilSeededSearchCatalog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dir)

	seedPath := filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_search.sql")
	seed, err := os.ReadFile(seedPath) //nolint:gosec // G304: test helper with literal relative path
	require.NoError(t, err)

	shardPath := filepath.Join(dir, "aws-ec2.db")
	require.NoError(t, catalog.BuildFromSQL(shardPath, string(seed)))
	return shardPath
}
