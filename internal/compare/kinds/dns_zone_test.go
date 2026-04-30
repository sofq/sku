package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedDNSZoneShard(t *testing.T, relPath string) *catalog.Catalog {
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

func TestQueryDNSZone_NoFilterReturnsAll(t *testing.T) {
	cat := seedDNSZoneShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_dns_zone_compare.sql"))
	rows, err := QueryDNSZone(context.Background(), cat, DNSZoneSpec{})
	require.NoError(t, err)
	require.Len(t, rows, 2, "both public and private rows should be returned with no mode filter")
}

func TestQueryDNSZone_ModeFilterNarrows(t *testing.T) {
	cat := seedDNSZoneShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_dns_zone_compare.sql"))
	rows, err := QueryDNSZone(context.Background(), cat, DNSZoneSpec{
		Mode: "private",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "mode=private should return only the private row")
	require.Equal(t, "private", rows[0].ResourceAttrs.Extra["mode"])
}

func TestQueryDNSZone_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedDNSZoneShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_dns_zone_compare.sql"))
	rows, err := QueryDNSZone(context.Background(), cat, DNSZoneSpec{
		Mode: "nonexistent",
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestQueryDNSZone_RegionFilterNarrows(t *testing.T) {
	cat := seedDNSZoneShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_dns_zone_compare.sql"))
	rows, err := QueryDNSZone(context.Background(), cat, DNSZoneSpec{
		Regions: []string{"global"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "global", r.Region)
	}
}
