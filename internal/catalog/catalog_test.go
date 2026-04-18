package catalog_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedShard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "openrouter.db")
	seed, err := os.ReadFile(filepath.Join("testdata", "seed.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(dst, string(seed)))
	return dst
}

func TestOpen_ReadsMetadata(t *testing.T) {
	path := seedShard(t)
	cat, err := catalog.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	require.Equal(t, "1", cat.SchemaVersion())
	require.Equal(t, "2026.04.18", cat.CatalogVersion())
	require.Equal(t, "USD", cat.Currency())
}

func TestLookupLLM_ReturnsAllServingProvidersByDefault_ExcludingAggregated(t *testing.T) {
	cat, err := catalog.Open(seedShard(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model: "anthropic/claude-opus-4.6",
	})
	require.NoError(t, err)
	require.Len(t, rows, 2, "aggregated row excluded by default")

	providers := []string{rows[0].Provider, rows[1].Provider}
	require.ElementsMatch(t, []string{"anthropic", "aws-bedrock"}, providers)

	for _, r := range rows {
		require.Equal(t, "anthropic/claude-opus-4.6", r.ResourceName)
		require.Len(t, r.Prices, 2)
		require.NotNil(t, r.Health)
	}
}

func TestLookupLLM_IncludeAggregatedFlag(t *testing.T) {
	cat, err := catalog.Open(seedShard(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model:             "anthropic/claude-opus-4.6",
		IncludeAggregated: true,
	})
	require.NoError(t, err)
	require.Len(t, rows, 3)

	// aggregated row should carry Aggregated=true and Health=nil
	var agg *catalog.Row
	for i := range rows {
		if rows[i].Provider == "openrouter" {
			agg = &rows[i]
		}
	}
	require.NotNil(t, agg)
	require.True(t, agg.Aggregated)
	require.Nil(t, agg.Health)
}

func TestLookupLLM_ServingProviderFilter(t *testing.T) {
	cat, err := catalog.Open(seedShard(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model:           "anthropic/claude-opus-4.6",
		ServingProvider: "aws-bedrock",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "aws-bedrock", rows[0].Provider)
}

func TestLookupLLM_NotFoundReturnsEmpty(t *testing.T) {
	cat, err := catalog.Open(seedShard(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model: "totally/made-up",
	})
	require.NoError(t, err)
	require.Empty(t, rows, "no match is not an error at the catalog layer")
}
