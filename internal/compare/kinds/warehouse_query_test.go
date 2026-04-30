package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedWarehouseQueryShard(t *testing.T, relPath string) *catalog.Catalog {
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

func TestQueryWarehouseQuery_DefaultModeIsOnDemand(t *testing.T) {
	cat := seedWarehouseQueryShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_warehouse_query_compare.sql"))
	rows, err := QueryWarehouseQuery(context.Background(), cat, WarehouseQuerySpec{
		Regions: []string{"bq-us"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "on-demand", rows[0].ResourceAttrs.Extra["mode"])
}

func TestQueryWarehouseQuery_CapacityModeWithEdition(t *testing.T) {
	cat := seedWarehouseQueryShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_warehouse_query_compare.sql"))
	rows, err := QueryWarehouseQuery(context.Background(), cat, WarehouseQuerySpec{
		Mode:    "capacity",
		Edition: "enterprise",
		Regions: []string{"bq-us"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "enterprise", rows[0].ResourceAttrs.Extra["edition"])
}

func TestQueryWarehouseQuery_StorageModeWithStorageTier(t *testing.T) {
	cat := seedWarehouseQueryShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_warehouse_query_compare.sql"))
	rows, err := QueryWarehouseQuery(context.Background(), cat, WarehouseQuerySpec{
		Mode:        "storage",
		StorageTier: "active",
		Regions:     []string{"bq-us"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "active", rows[0].ResourceAttrs.Extra["storage_tier"])
}

func TestQueryWarehouseQuery_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedWarehouseQueryShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_warehouse_query_compare.sql"))
	rows, err := QueryWarehouseQuery(context.Background(), cat, WarehouseQuerySpec{
		Mode:    "nonexistent",
		Regions: []string{"bq-us"},
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}
