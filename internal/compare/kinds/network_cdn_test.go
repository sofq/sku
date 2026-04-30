package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedNetworkCDNShard(t *testing.T, relPath string) *catalog.Catalog {
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

func TestQueryNetworkCDN_NoFilterReturnsAll(t *testing.T) {
	cat := seedNetworkCDNShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_network_cdn_compare.sql"))
	rows, err := QueryNetworkCDN(context.Background(), cat, NetworkCDNSpec{})
	require.NoError(t, err)
	require.Len(t, rows, 2, "both edge-egress and origin-shield rows should be returned with no mode filter")
}

func TestQueryNetworkCDN_ModeFilterNarrows(t *testing.T) {
	cat := seedNetworkCDNShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_network_cdn_compare.sql"))
	rows, err := QueryNetworkCDN(context.Background(), cat, NetworkCDNSpec{
		Mode: "origin-shield",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "mode=origin-shield should return only that row")
	require.Equal(t, "origin-shield", rows[0].ResourceAttrs.Extra["mode"])
}

func TestQueryNetworkCDN_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedNetworkCDNShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_network_cdn_compare.sql"))
	rows, err := QueryNetworkCDN(context.Background(), cat, NetworkCDNSpec{
		Mode: "nonexistent",
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestQueryNetworkCDN_RegionFilterNarrows(t *testing.T) {
	cat := seedNetworkCDNShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_network_cdn_compare.sql"))
	rows, err := QueryNetworkCDN(context.Background(), cat, NetworkCDNSpec{
		Regions: []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "us-east-1", r.Region)
	}
}
