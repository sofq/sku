package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedStorageObjectShard(t *testing.T, rel string) *catalog.Catalog {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "shard.db")
	data, err := readSQL(rel)
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(path, data))
	cat, err := catalog.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestQueryStorageObject_filtersByStorageClass(t *testing.T) {
	cat := seedStorageObjectShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_aws_m3a2.sql"))
	rows, err := QueryStorageObject(context.Background(), cat, StorageObjectSpec{
		StorageClass: "standard",
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "storage.object", r.Kind)
		require.Equal(t, "standard", r.ResourceName)
	}
}
