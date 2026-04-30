package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedSearchEngineShard(t *testing.T, relPath string) *catalog.Catalog {
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

func TestQuerySearchEngine_DefaultModeReturnsManagedCluster(t *testing.T) {
	cat := seedSearchEngineShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_search_engine_compare.sql"))
	rows, err := QuerySearchEngine(context.Background(), cat, SearchEngineSpec{
		Regions: []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "managed-cluster", r.ResourceAttrs.Extra["mode"])
	}
}

func TestQuerySearchEngine_ServerlessMode(t *testing.T) {
	cat := seedSearchEngineShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_search_engine_compare.sql"))
	rows, err := QuerySearchEngine(context.Background(), cat, SearchEngineSpec{
		Mode:    "serverless",
		Regions: []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "serverless", r.ResourceAttrs.Extra["mode"])
	}
}

func TestQuerySearchEngine_InstanceTypeFilter(t *testing.T) {
	cat := seedSearchEngineShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_search_engine_compare.sql"))
	rows, err := QuerySearchEngine(context.Background(), cat, SearchEngineSpec{
		InstanceType: "r6g.large.search",
		Regions:      []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "r6g.large.search", rows[0].ResourceName)
}

func TestQuerySearchEngine_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedSearchEngineShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_search_engine_compare.sql"))
	rows, err := QuerySearchEngine(context.Background(), cat, SearchEngineSpec{
		InstanceType: "nonexistent",
		Regions:      []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestQuerySearchEngine_MinVCPUAndMemoryFilter(t *testing.T) {
	cat := seedSearchEngineShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_search_engine_compare.sql"))
	rows, err := QuerySearchEngine(context.Background(), cat, SearchEngineSpec{
		MinVCPU:     4,
		MinMemoryGB: 32,
		Regions:     []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "r6g.xlarge.search", rows[0].ResourceName)
}
