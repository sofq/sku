package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedPaasAppShard(t *testing.T, relPath string) *catalog.Catalog {
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

func TestQueryPaasApp_DefaultOSIsLinux(t *testing.T) {
	cat := seedPaasAppShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_paas_app_compare.sql"))
	rows, err := QueryPaasApp(context.Background(), cat, PaasAppSpec{
		Regions: []string{"eastus"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "linux", r.Terms.OS)
	}
}

func TestQueryPaasApp_WindowsOSFilter(t *testing.T) {
	cat := seedPaasAppShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_paas_app_compare.sql"))
	rows, err := QueryPaasApp(context.Background(), cat, PaasAppSpec{
		PlanOS:  "windows",
		Regions: []string{"eastus"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "windows", r.Terms.OS)
	}
}

func TestQueryPaasApp_TierFilter(t *testing.T) {
	cat := seedPaasAppShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_paas_app_compare.sql"))
	rows, err := QueryPaasApp(context.Background(), cat, PaasAppSpec{
		Tier:    "premiumv3",
		Regions: []string{"eastus"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "premiumv3", r.Terms.SupportTier)
		require.Equal(t, "linux", r.Terms.OS)
	}
}

func TestQueryPaasApp_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedPaasAppShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_paas_app_compare.sql"))
	rows, err := QueryPaasApp(context.Background(), cat, PaasAppSpec{
		Tier:    "nonexistent",
		Regions: []string{"eastus"},
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}
