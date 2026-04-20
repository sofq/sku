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

func TestQueryStorageObject_durabilityFilterExcludesLowerTiers(t *testing.T) {
	cat := seedStorageObjectShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_aws_m3a2.sql"))
	rows, err := QueryStorageObject(context.Background(), cat, StorageObjectSpec{
		DurabilityNines: 12, // seed rows have 11 nines
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestQueryStorageObject_availabilityTierFilter(t *testing.T) {
	cat := seedStorageObjectShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_aws_m3a2.sql"))
	rows, err := QueryStorageObject(context.Background(), cat, StorageObjectSpec{
		AvailabilityTier: "infrequent",
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.NotNil(t, r.ResourceAttrs.AvailabilityTier)
		require.Equal(t, "infrequent", *r.ResourceAttrs.AvailabilityTier)
	}
}

func TestQueryStorageObject_regionFilter(t *testing.T) {
	cat := seedStorageObjectShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_aws_m3a2.sql"))
	rows, err := QueryStorageObject(context.Background(), cat, StorageObjectSpec{
		Regions: []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "us-east-1", r.Region)
	}
}
