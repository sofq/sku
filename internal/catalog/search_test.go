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

func TestSearch_KindFilter(t *testing.T) {
	cat := openSeededSearch(t)
	rows, err := cat.Search(context.Background(), catalog.SearchFilter{
		Provider: "aws", Service: "ec2", Kind: "db.relational",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "db.m5.large", rows[0].ResourceName)
}

func TestSearch_RegionFilter(t *testing.T) {
	cat := openSeededSearch(t)
	rows, err := cat.Search(context.Background(), catalog.SearchFilter{
		Provider: "aws", Service: "ec2", Region: "us-west-2",
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, r := range rows {
		require.Equal(t, "us-west-2", r.Region)
	}
}

func TestSearch_ResourceNameFilter(t *testing.T) {
	cat := openSeededSearch(t)
	rows, err := cat.Search(context.Background(), catalog.SearchFilter{
		Provider: "aws", Service: "ec2", ResourceName: "m5.large",
	})
	require.NoError(t, err)
	// m5.large exists in both us-east-1 and us-west-2 => 2 rows.
	require.Len(t, rows, 2)
}
