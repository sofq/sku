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

func TestTenanciesForEngine(t *testing.T) {
	tests := []struct {
		engine string
		want   []string
	}{
		{"oracle", []string{"oracle"}},
		{"exotic-db", []string{"exotic-db"}},
	}
	for _, tc := range tests {
		got := tenanciesForEngine(tc.engine)
		require.Equal(t, tc.want, got, "engine=%s", tc.engine)
	}
}

func TestQueryDBRelational_postgresMatchesAzureHostedDBTenancy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shard.db")
	base, err := readSQL(filepath.Join("..", "..", "catalog", "testdata", "seed_azure.sql"))
	require.NoError(t, err)
	const azurePostgresRow = `
INSERT INTO skus VALUES
  ('azure-pg-gen5-2-eastus-flex', 'azure','postgres','db.relational','Gen5 2 vCore','eastus','us-east','dummy');
INSERT INTO terms VALUES
  ('azure-pg-gen5-2-eastus-flex', 'on_demand','azure-postgres','flexible-server','','','');
INSERT INTO resource_attrs (sku_id, vcpu, memory_gb, architecture, extra) VALUES
  ('azure-pg-gen5-2-eastus-flex', 2, 8.0, NULL, '{"deployment_option":"flexible-server"}');
INSERT INTO prices VALUES
  ('azure-pg-gen5-2-eastus-flex', 'compute','',0.12,'hrs');
`
	require.NoError(t, catalog.BuildFromSQL(path, base+azurePostgresRow))
	cat, err := catalog.Open(path)
	require.NoError(t, err)
	defer func() { _ = cat.Close() }()

	rows, err := QueryDBRelational(context.Background(), cat, DBRelationalSpec{
		Engine:           "postgres",
		DeploymentOption: "flexible-server",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "azure-postgres", rows[0].Terms.Tenancy)
}
