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
