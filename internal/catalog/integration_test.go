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

func TestIntegration_S3PointLookup(t *testing.T) {
	dir := os.Getenv("SKU_TEST_SHARD_DIR")
	if dir == "" {
		t.Skip("SKU_TEST_SHARD_DIR not set")
	}
	cat, err := catalog.Open(filepath.Join(dir, "aws-s3.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupStorageObject(context.Background(), catalog.StorageObjectFilter{
		Provider: "aws", Service: "s3",
		StorageClass: "standard", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Prices, 3)
}

func TestIntegration_LambdaPointLookup(t *testing.T) {
	dir := os.Getenv("SKU_TEST_SHARD_DIR")
	if dir == "" {
		t.Skip("SKU_TEST_SHARD_DIR not set")
	}
	cat, err := catalog.Open(filepath.Join(dir, "aws-lambda.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupServerlessFunction(context.Background(), catalog.ServerlessFunctionFilter{
		Provider: "aws", Service: "lambda",
		Architecture: "arm64", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestIntegration_EBSPointLookup(t *testing.T) {
	dir := os.Getenv("SKU_TEST_SHARD_DIR")
	if dir == "" {
		t.Skip("SKU_TEST_SHARD_DIR not set")
	}
	cat, err := catalog.Open(filepath.Join(dir, "aws-ebs.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupStorageBlock(context.Background(), catalog.StorageBlockFilter{
		Provider: "aws", Service: "ebs",
		VolumeType: "gp3", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestIntegration_DynamoDBPointLookup(t *testing.T) {
	dir := os.Getenv("SKU_TEST_SHARD_DIR")
	if dir == "" {
		t.Skip("SKU_TEST_SHARD_DIR not set")
	}
	cat, err := catalog.Open(filepath.Join(dir, "aws-dynamodb.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupNoSQLDB(context.Background(), catalog.NoSQLDBFilter{
		Provider: "aws", Service: "dynamodb",
		ResourceName: "standard", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Prices, 3)
}

func TestIntegration_CloudFrontPointLookup(t *testing.T) {
	dir := os.Getenv("SKU_TEST_SHARD_DIR")
	if dir == "" {
		t.Skip("SKU_TEST_SHARD_DIR not set")
	}
	cat, err := catalog.Open(filepath.Join(dir, "aws-cloudfront.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupCDN(context.Background(), catalog.CDNFilter{
		Provider: "aws", Service: "cloudfront",
		ResourceName: "standard", Region: "eu-west-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "data_transfer_out", rows[0].Prices[0].Dimension)
}

func TestIntegration_AzureVMPointLookup(t *testing.T) {
	dir := os.Getenv("SKU_TEST_SHARD_DIR")
	if dir == "" {
		t.Skip("SKU_TEST_SHARD_DIR not set")
	}
	cat, err := catalog.Open(filepath.Join(dir, "azure-vm.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider: "azure", Service: "vm",
		InstanceType: "Standard_D2_v3", Region: "eastus",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "linux"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestIntegration_AzureSQLPointLookup(t *testing.T) {
	dir := os.Getenv("SKU_TEST_SHARD_DIR")
	if dir == "" {
		t.Skip("SKU_TEST_SHARD_DIR not set")
	}
	cat, err := catalog.Open(filepath.Join(dir, "azure-sql.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider: "azure", Service: "sql",
		InstanceType: "GP_Gen5_2", Region: "eastus",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "azure-sql", OS: "single-az"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
