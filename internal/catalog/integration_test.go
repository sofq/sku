//go:build integration

package catalog_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func TestIntegration_RealBuiltShard(t *testing.T) {
	path := os.Getenv("SKU_TEST_SHARD")
	if path == "" {
		t.Skip("SKU_TEST_SHARD not set; run `make openrouter-shard && SKU_TEST_SHARD=... go test -tags=integration`")
	}
	cat, err := catalog.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	require.Equal(t, "USD", cat.Currency())

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model: "anthropic/claude-opus-4.6",
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	// Every row has at least one price dimension.
	for _, r := range rows {
		require.NotEmpty(t, r.Prices, "row %s has no prices", r.SKUID)
	}
}
