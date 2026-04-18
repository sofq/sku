package catalog_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func openSeededSearch(t *testing.T) *catalog.Catalog {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "aws-ec2.db")
	seed, err := os.ReadFile(filepath.Join("testdata", "seed_search.sql")) //nolint:gosec // G304: test helper with literal basename
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(dst, string(seed)))
	cat, err := catalog.Open(dst)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestSearch_RequiresProviderAndService(t *testing.T) {
	cat := openSeededSearch(t)

	_, err := cat.Search(context.Background(), catalog.SearchFilter{Service: "ec2"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Provider")

	_, err = cat.Search(context.Background(), catalog.SearchFilter{Provider: "aws"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Service")
}

func TestSearch_BaseQueryReturnsAllRows(t *testing.T) {
	cat := openSeededSearch(t)

	rows, err := cat.Search(context.Background(), catalog.SearchFilter{
		Provider: "aws", Service: "ec2",
	})
	require.NoError(t, err)
	require.Len(t, rows, 8)

	// Default sort is resource_name + sku_id; the first row is the alphabetically
	// lowest resource_name in the fixture (c5.large).
	require.Equal(t, "c5.large", rows[0].ResourceName)
	// Every row carries currency from metadata and catalog_version.
	for _, r := range rows {
		require.Equal(t, "USD", r.Currency)
		require.Equal(t, "2026.04.18", r.CatalogVersion)
		require.NotEmpty(t, r.Prices, "row %s has no prices", r.SKUID)
	}
}
