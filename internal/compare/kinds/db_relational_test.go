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

func TestQueryDBRelational_multiAZExcludesSingleAZ(t *testing.T) {
	cat := seedDBRelationalShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_aws.sql"))
	rows, err := QueryDBRelational(context.Background(), cat, DBRelationalSpec{
		Engine: "postgres", DeploymentOption: "multi-az",
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "multi-az", r.Terms.OS)
	}
}

func TestQueryDBRelational_vcpuPredicateFiltersUnderspec(t *testing.T) {
	cat := seedDBRelationalShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_aws.sql"))
	rows, err := QueryDBRelational(context.Background(), cat, DBRelationalSpec{
		Engine: "postgres", DeploymentOption: "single-az",
		VCPU: 64,
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestQueryDBRelational_requiresEngineAndDeploymentOption(t *testing.T) {
	cat := seedDBRelationalShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_aws.sql"))
	_, err := QueryDBRelational(context.Background(), cat, DBRelationalSpec{
		Engine: "",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "engine")
}
