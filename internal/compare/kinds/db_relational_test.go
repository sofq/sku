package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedDBRelationalShard(t *testing.T, rel string) *catalog.Catalog {
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

func TestQueryDBRelational_pinsPostgresSingleAZByDefault(t *testing.T) {
	cat := seedDBRelationalShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_aws.sql"))
	rows, err := QueryDBRelational(context.Background(), cat, DBRelationalSpec{
		VCPU: 2, MemoryGB: 8,
		Engine:           "postgres",
		DeploymentOption: "single-az",
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "db.relational", r.Kind)
		require.Equal(t, "postgres", r.Terms.Tenancy)
		require.Equal(t, "single-az", r.Terms.OS)
	}
}
