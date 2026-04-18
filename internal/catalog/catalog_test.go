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
	return seedShardFromFile(t, "seed.sql", "openrouter.db")
}

func seedShardFromFile(t *testing.T, name, dbName string) string {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, dbName)
	seed, err := os.ReadFile(filepath.Join("testdata", name)) //nolint:gosec // G304: test helper with literal basename
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(dst, string(seed)))
	return dst
}

func openSeededAWS(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_aws.sql", "aws-ec2.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func openSeededAWSM3a2(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_aws_m3a2.sql", "aws-s3.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
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

func TestLookupVM_PointLookup(t *testing.T) {
	cat := openSeededAWS(t)
	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider:     "aws",
		Service:      "ec2",
		InstanceType: "m5.large",
		Region:       "us-east-1",
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "linux"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "m5.large", rows[0].ResourceName)
	require.Equal(t, "us-east-1", rows[0].Region)
	require.Len(t, rows[0].Prices, 1)
	require.Equal(t, 0.096, rows[0].Prices[0].Amount)
}

func TestLookupVM_ListByInstance(t *testing.T) {
	cat := openSeededAWS(t)
	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider: "aws", Service: "ec2", InstanceType: "m5.large",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "linux"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2, "both seed regions should come back when region is unset")
}

func TestLookupDBRelational_PointLookup(t *testing.T) {
	cat := openSeededAWS(t)
	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider: "aws", Service: "rds",
		InstanceType: "db.m5.large",
		Region:       "us-east-1",
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: "postgres", OS: "multi-az"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 0.300, rows[0].Prices[0].Amount)
}

func TestLookupStorageObject_PointLookup(t *testing.T) {
	cat := openSeededAWSM3a2(t)

	rows, err := cat.LookupStorageObject(context.Background(), catalog.StorageObjectFilter{
		Provider: "aws", Service: "s3",
		StorageClass: "standard", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Prices, 3)
}

func TestLookupServerlessFunction_PointLookup(t *testing.T) {
	cat := openSeededAWSM3a2(t)

	rows, err := cat.LookupServerlessFunction(context.Background(), catalog.ServerlessFunctionFilter{
		Provider: "aws", Service: "lambda",
		Architecture: "arm64", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	var dur float64
	for _, p := range rows[0].Prices {
		if p.Dimension == "duration" {
			dur = p.Amount
		}
	}
	require.Equal(t, 0.0000133334, dur)
}

func TestLookupStorageBlock_PointLookup(t *testing.T) {
	cat := openSeededAWSM3a2(t)

	rows, err := cat.LookupStorageBlock(context.Background(), catalog.StorageBlockFilter{
		Provider: "aws", Service: "ebs",
		VolumeType: "gp3", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 0.08, rows[0].Prices[0].Amount)
}
