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

func openSeededAWSM3a3(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_aws_m3a3.sql", "aws-dynamodb.db"))
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

func TestLookupNoSQLDB_SeededStandardUSE1(t *testing.T) {
	cat := openSeededAWSM3a3(t)

	rows, err := cat.LookupNoSQLDB(context.Background(), catalog.NoSQLDBFilter{
		Provider: "aws", Service: "dynamodb",
		ResourceName: "standard", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	dims := map[string]float64{}
	for _, p := range rows[0].Prices {
		dims[p.Dimension] = p.Amount
	}
	require.Equal(t, 0.25, dims["storage"])
	require.Equal(t, 0.000000125, dims["read_request_units"])
	require.Equal(t, 0.000000625, dims["write_request_units"])
}

func TestLookupCDN_SeededEUWest(t *testing.T) {
	cat := openSeededAWSM3a3(t)

	rows, err := cat.LookupCDN(context.Background(), catalog.CDNFilter{
		Provider: "aws", Service: "cloudfront",
		ResourceName: "standard", Region: "eu-west-1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Prices, 1)
	require.Equal(t, "data_transfer_out", rows[0].Prices[0].Dimension)
	require.Equal(t, 0.085, rows[0].Prices[0].Amount)
}

func TestLookupNoSQLDB_MissingResourceNameErrors(t *testing.T) {
	cat := openSeededAWSM3a3(t)
	_, err := cat.LookupNoSQLDB(context.Background(), catalog.NoSQLDBFilter{
		Provider: "aws", Service: "dynamodb",
		Region: "us-east-1",
		Terms:  catalog.Terms{Commitment: "on_demand"},
	})
	require.Error(t, err)
}

func openSeededAzure(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_azure.sql", "azure-vm.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupVM_AzurePointLookup(t *testing.T) {
	cat := openSeededAzure(t)
	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider:     "azure",
		Service:      "vm",
		InstanceType: "Standard_D2_v3",
		Region:       "eastus",
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "linux"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Standard_D2_v3", rows[0].ResourceName)
	require.Equal(t, "eastus", rows[0].Region)
	require.Equal(t, 0.096, rows[0].Prices[0].Amount)
}

func TestLookupVM_AzureWindowsDifferentTermsHash(t *testing.T) {
	cat := openSeededAzure(t)
	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider:     "azure",
		Service:      "vm",
		InstanceType: "Standard_D2_v3",
		Region:       "eastus",
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "windows"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 0.144, rows[0].Prices[0].Amount)
}

func TestLookupDBRelational_AzureSQLManagedInstance(t *testing.T) {
	cat := openSeededAzure(t)
	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider:     "azure",
		Service:      "sql",
		InstanceType: "BC_Gen5_2",
		Region:       "eastus",
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: "azure-sql", OS: "managed-instance"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 1.058, rows[0].Prices[0].Amount)
}

func openSeededGCP(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_gcp.sql", "gcp-gce.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupVM_GCPPointLookup(t *testing.T) {
	cat := openSeededGCP(t)
	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider: "gcp", Service: "gce",
		InstanceType: "n1-standard-2", Region: "us-east1",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "linux"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "n1-standard-2", rows[0].ResourceName)
	require.Equal(t, "us-east", rows[0].RegionGroup)
	require.InDelta(t, 0.095, rows[0].Prices[0].Amount, 1e-9)
}

func TestLookupDBRelational_GCPCloudSQLZonal(t *testing.T) {
	cat := openSeededGCP(t)
	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider: "gcp", Service: "cloud-sql",
		InstanceType: "db-custom-2-7680", Region: "us-east1",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "cloud-sql-postgres", OS: "zonal"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.InDelta(t, 0.115, rows[0].Prices[0].Amount, 1e-9)
}

func TestLookupDBRelational_GCPCloudSQLRegionalDifferentTermsHash(t *testing.T) {
	cat := openSeededGCP(t)
	zonal, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider: "gcp", Service: "cloud-sql",
		InstanceType: "db-custom-2-7680", Region: "us-east1",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "cloud-sql-postgres", OS: "zonal"},
	})
	require.NoError(t, err)
	regional, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider: "gcp", Service: "cloud-sql",
		InstanceType: "db-custom-2-7680", Region: "us-east1",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "cloud-sql-postgres", OS: "regional"},
	})
	require.NoError(t, err)
	require.Len(t, zonal, 1)
	require.Len(t, regional, 1)
	require.NotEqual(t, zonal[0].TermsHash, regional[0].TermsHash,
		"zonal and regional must hash to different terms rows")
	require.Greater(t, regional[0].Prices[0].Amount, zonal[0].Prices[0].Amount)
}

func openSeededGCPM3b4(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_gcp_m3b4.sql", "gcp-gcs.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupStorageObject_GCPGCSPointLookup(t *testing.T) {
	cat := openSeededGCPM3b4(t)
	rows, err := cat.LookupStorageObject(context.Background(), catalog.StorageObjectFilter{
		Provider: "gcp", Service: "gcs",
		StorageClass: "standard", Region: "us-east1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "standard", rows[0].ResourceName)
	dims := map[string]bool{}
	for _, p := range rows[0].Prices {
		dims[p.Dimension] = true
	}
	for _, want := range []string{"storage", "read-ops", "write-ops"} {
		require.Truef(t, dims[want], "missing dimension %s", want)
	}
}

func TestLookupStorageObject_GCPGCSRegionOptional(t *testing.T) {
	cat := openSeededGCPM3b4(t)
	rows, err := cat.LookupStorageObject(context.Background(), catalog.StorageObjectFilter{
		Provider: "gcp", Service: "gcs",
		StorageClass: "standard",
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "standard class exists only in us-east1 in the seed")
}

func TestLookupServerlessFunction_GCPRunPointLookup(t *testing.T) {
	cat := openSeededGCPM3b4(t)
	rows, err := cat.LookupServerlessFunction(context.Background(), catalog.ServerlessFunctionFilter{
		Provider: "gcp", Service: "run",
		Architecture: "x86_64", Region: "europe-west1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "run", rows[0].Service)
	require.Equal(t, "x86_64", rows[0].ResourceName)
}

func TestLookupServerlessFunction_GCPFunctionsPointLookup(t *testing.T) {
	cat := openSeededGCPM3b4(t)
	rows, err := cat.LookupServerlessFunction(context.Background(), catalog.ServerlessFunctionFilter{
		Provider: "gcp", Service: "functions",
		Architecture: "x86_64", Region: "us-east1",
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "functions", rows[0].Service)
	require.Equal(t, "x86_64", rows[0].ResourceName)
}

func openSeededCacheKV(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_cache_kv.sql", "aws-elasticache.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupCacheKV_ByResourceNameAndRegion(t *testing.T) {
	cat := openSeededCacheKV(t)
	rows, err := cat.LookupCacheKV(context.Background(), catalog.CacheKVFilter{
		Provider: "aws", Service: "elasticache",
		ResourceName: "cache.r6g.large", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "redis"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "cache.r6g.large", rows[0].ResourceName)
	require.NotNil(t, rows[0].ResourceAttrs.MemoryGB)
}

func TestLookupCacheKV_MissingResourceNameErrors(t *testing.T) {
	cat := openSeededCacheKV(t)
	_, err := cat.LookupCacheKV(context.Background(), catalog.CacheKVFilter{
		Provider: "aws", Service: "elasticache",
		Region: "us-east-1",
		Terms:  catalog.Terms{Commitment: "on_demand", Tenancy: "redis"},
	})
	require.Error(t, err)
}

func openSeededContainerOrchestration(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_container_orchestration.sql", "aws-eks.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupContainerOrchestration_ByResourceNameAndRegion(t *testing.T) {
	cat := openSeededContainerOrchestration(t)
	rows, err := cat.LookupContainerOrchestration(context.Background(), catalog.ContainerOrchestrationFilter{
		Provider: "aws", Service: "eks",
		ResourceName: "eks-standard", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "kubernetes", OS: "standard"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "eks-standard", rows[0].ResourceName)
}

func TestLookupContainerOrchestration_MissingResourceName(t *testing.T) {
	cat := openSeededContainerOrchestration(t)
	_, err := cat.LookupContainerOrchestration(context.Background(), catalog.ContainerOrchestrationFilter{
		Provider: "aws", Service: "eks",
	})
	require.Error(t, err)
}

func openSeededSearchEngine(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_aws_opensearch.sql", "aws-opensearch.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupSearchEngine_ByResourceNameAndRegion(t *testing.T) {
	cat := openSeededSearchEngine(t)
	rows, err := cat.LookupSearchEngine(context.Background(), catalog.SearchEngineFilter{
		Provider: "aws", Service: "opensearch",
		ResourceName: "r6g.large.search", Region: "us-east-1",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "managed-cluster"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "r6g.large.search", rows[0].ResourceName)
	require.NotNil(t, rows[0].ResourceAttrs.MemoryGB)
}

func TestLookupSearchEngine_MissingResourceNameErrors(t *testing.T) {
	cat := openSeededSearchEngine(t)
	_, err := cat.LookupSearchEngine(context.Background(), catalog.SearchEngineFilter{
		Provider: "aws", Service: "opensearch",
	})
	require.Error(t, err)
}

func openSeededPaasApp(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_azure_appservice.sql", "azure-appservice.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupPaasApp_ByResourceNameAndRegion(t *testing.T) {
	cat := openSeededPaasApp(t)
	rows, err := cat.LookupPaasApp(context.Background(), catalog.PaasAppFilter{
		Provider: "azure", Service: "appservice",
		ResourceName: "P1v3", Region: "eastus",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "dedicated", OS: "linux", SupportTier: "premiumv3"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "P1v3", rows[0].ResourceName)
	require.Equal(t, "linux", rows[0].Terms.OS)
}

func TestLookupPaasApp_TierOnlyQuery(t *testing.T) {
	cat := openSeededPaasApp(t)
	// No ResourceName — returns all plans in premiumv3 tier.
	rows, err := cat.LookupPaasApp(context.Background(), catalog.PaasAppFilter{
		Provider: "azure", Service: "appservice", Region: "eastus",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "dedicated", SupportTier: "premiumv3"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2) // linux + windows P1v3
}

func openSeededWarehouseQuery(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_gcp_bigquery.sql", "gcp-bigquery.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupWarehouseQuery_ByResourceNameAndRegion(t *testing.T) {
	cat := openSeededWarehouseQuery(t)
	rows, err := cat.LookupWarehouseQuery(context.Background(), catalog.WarehouseQueryFilter{
		Provider: "gcp", Service: "bigquery",
		ResourceName: "on-demand", Region: "bq-us",
		Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "on-demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "on-demand", rows[0].ResourceName)
}

func TestLookupWarehouseQuery_MissingResourceNameErrors(t *testing.T) {
	cat := openSeededWarehouseQuery(t)
	_, err := cat.LookupWarehouseQuery(context.Background(), catalog.WarehouseQueryFilter{
		Provider: "gcp", Service: "bigquery",
	})
	require.Error(t, err)
}
