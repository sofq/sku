package sku

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompare_crossProvider_minVCPU(t *testing.T) {
	testutilSeededComputeVMCatalog(t)
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "compute.vm", "--vcpu", "2", "--memory", "4", "--regions", "us-east", "--limit", "5"})
	require.NoError(t, cmd.Execute())

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	require.NotEmpty(t, lines)
	providersSeen := map[string]bool{}
	for _, ln := range lines {
		var env map[string]any
		require.NoError(t, json.Unmarshal([]byte(ln), &env))
		if p, ok := env["provider"].(string); ok {
			providersSeen[p] = true
		}
		loc, _ := env["location"].(map[string]any)
		require.NotNil(t, loc, "compare preset must include location.normalized_region")
		require.NotEmpty(t, loc["normalized_region"])
	}
	require.GreaterOrEqual(t, len(providersSeen), 2, "fan-out should surface at least two providers; got %v", providersSeen)
}

func TestCompare_unsupportedKind(t *testing.T) {
	testutilSeededComputeVMCatalog(t)
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "llm.text"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, stderr.String(), "llm.text")
}

func TestCompareCmd_storageObjectEndToEnd(t *testing.T) {
	dir := testutilSeededStorageObjectCatalog(t)
	t.Setenv("SKU_DATA_DIR", dir)
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "storage.object",
		"--storage-class", "standard", "--sort", "price", "--json", "--limit", "10"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"provider":"aws"`)
}

func TestCompareCmd_storageObjectRejectsVCPU(t *testing.T) {
	testutilSeededStorageObjectCatalog(t)
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "storage.object",
		"--storage-class", "standard", "--vcpu", "4"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, stderr.String(), `"reason":"flag_invalid"`)
}

func TestCompareCmd_dryRunStorageObject(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "storage.object",
		"--storage-class", "standard", "--availability-tier", "standard",
		"--regions", "us-east", "--dry-run"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"command":"compare"`)
	require.Contains(t, stdout.String(), `"storage_class":"standard"`)
	require.Contains(t, stdout.String(), `"availability_tier":"standard"`)
	require.Contains(t, stdout.String(), `"aws-s3"`)
}

func TestCompareCmd_dbRelationalEndToEnd(t *testing.T) {
	dir := testutilSeededDBRelationalCatalog(t)
	t.Setenv("SKU_DATA_DIR", dir)
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "db.relational",
		"--vcpu", "2", "--memory", "8",
		"--engine", "postgres", "--deployment-option", "single-az",
		"--sort", "price", "--json", "--limit", "10"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"kind":"db.relational"`)
	require.Contains(t, stdout.String(), `"commitment":"on_demand"`)
}

func TestCompareCmd_dbRelationalNoRowsExitsNotFound(t *testing.T) {
	dir := testutilSeededDBRelationalCatalog(t)
	t.Setenv("SKU_DATA_DIR", dir)
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "db.relational",
		"--vcpu", "128", "--engine", "postgres", "--deployment-option", "single-az"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, stderr.String(), `"not_found"`)
	// B11: --max-price was not passed, so it must NOT be echoed back as
	// an applied filter (shipping `max_price: 0` would claim the comparator
	// filtered on "free only", which is a lie).
	require.NotContains(t, stderr.String(), `"max_price":0`)
	require.NotContains(t, stderr.String(), `"max_price": 0`)
}

func TestCompareCmd_maxPriceEchoedWhenSet(t *testing.T) {
	dir := testutilSeededDBRelationalCatalog(t)
	t.Setenv("SKU_DATA_DIR", dir)
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	// Passing --max-price 0.0001 with huge --vcpu forces NotFound; the
	// applied-filters echo should then include max_price.
	cmd.SetArgs([]string{"compare", "--kind", "db.relational",
		"--vcpu", "128", "--max-price", "0.0001",
		"--engine", "postgres", "--deployment-option", "single-az"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, stderr.String(), `"max_price":0.0001`)
}

func TestCompareCmd_dryRunDBRelational(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "db.relational",
		"--vcpu", "2", "--memory", "8",
		"--engine", "postgres", "--deployment-option", "multi-az",
		"--regions", "us-east", "--dry-run"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"engine":"postgres"`)
	require.Contains(t, stdout.String(), `"deployment_option":"multi-az"`)
	require.Contains(t, stdout.String(), `"aws-rds"`)
}

func TestCompareCmd_dryRunDBRelationalIncludesAzureHostedDBShards(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "db.relational", "--dry-run"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"azure-postgres"`)
	require.Contains(t, stdout.String(), `"azure-mysql"`)
	require.Contains(t, stdout.String(), `"azure-mariadb"`)
}

func TestCompareCmd_dryRunCacheKVIncludesCacheShards(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "cache.kv",
		"--memory", "16", "--engine", "memcached",
		"--regions", "us-east", "--dry-run"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"kind":"cache.kv"`)
	require.Contains(t, stdout.String(), `"memory_gb":16`)
	require.Contains(t, stdout.String(), `"engine":"memcached"`)
	require.Contains(t, stdout.String(), `"aws-elasticache"`)
	require.Contains(t, stdout.String(), `"azure-redis"`)
	require.Contains(t, stdout.String(), `"gcp-memorystore"`)
}

func TestCompare_dryRun(t *testing.T) {
	testutilSeededComputeVMCatalog(t)
	var stdout bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"compare", "--kind", "compute.vm", "--vcpu", "2", "--dry-run"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, stdout.String(), `"dry_run":true`)
	require.Contains(t, stdout.String(), `"command":"compare"`)
}
