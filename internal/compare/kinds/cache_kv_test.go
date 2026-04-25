package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedCacheKVShard(t *testing.T, relPath string) *catalog.Catalog {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "shard.db")
	data, err := readSQL(relPath)
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(path, data))
	cat, err := catalog.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestQueryCacheKV_FiltersByMemoryWindow(t *testing.T) {
	cat := seedCacheKVShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_cache_kv_compare.sql"))
	rows, err := QueryCacheKV(context.Background(), cat, CacheKVSpec{
		MemoryGB: 16,
		Regions:  []string{"us-east-1"},
	})
	require.NoError(t, err)
	// Rows with 15 GB are in [14.4, 24.0]; row with 26 GB is out.
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.NotNil(t, r.ResourceAttrs.MemoryGB)
		m := *r.ResourceAttrs.MemoryGB
		require.GreaterOrEqual(t, m, 16.0*0.9)
		require.LessOrEqual(t, m, 16.0*1.5)
	}
}

func TestQueryCacheKV_EngineFilter(t *testing.T) {
	cat := seedCacheKVShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_cache_kv_compare.sql"))
	rows, err := QueryCacheKV(context.Background(), cat, CacheKVSpec{
		MemoryGB: 16,
		Engine:   "memcached",
		Regions:  []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "memcached", r.Terms.Tenancy)
	}
}

func TestQueryCacheKV_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedCacheKVShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_cache_kv_compare.sql"))
	rows, err := QueryCacheKV(context.Background(), cat, CacheKVSpec{
		MemoryGB: 9999,
		Regions:  []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}
