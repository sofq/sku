//go:build integration

package catalog_test

import (
	"context"
	"os"
	"path/filepath"
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

func TestIntegration_EC2PointLookup(t *testing.T) {
	dir := os.Getenv("SKU_TEST_SHARD_DIR")
	if dir == "" {
		t.Skip("SKU_TEST_SHARD_DIR not set")
	}
	cat, err := catalog.Open(filepath.Join(dir, "aws-ec2.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider: "aws", Service: "ec2",
		InstanceType: "m5.large", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "linux"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestIntegration_RDSPointLookup(t *testing.T) {
	dir := os.Getenv("SKU_TEST_SHARD_DIR")
	if dir == "" {
		t.Skip("SKU_TEST_SHARD_DIR not set")
	}
	cat, err := catalog.Open(filepath.Join(dir, "aws-rds.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider: "aws", Service: "rds",
		InstanceType: "db.m5.large", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "postgres", OS: "single-az"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
