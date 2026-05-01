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
	require.Contains(t, stdout.String(), `"aws-aurora"`)
	require.Contains(t, stdout.String(), `"azure-postgres"`)
	require.Contains(t, stdout.String(), `"azure-mysql"`)
	require.Contains(t, stdout.String(), `"azure-mariadb"`)
	require.Contains(t, stdout.String(), `"gcp-spanner"`)
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

func TestCompareContainerOrchestrationDryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "container.orchestration",
		"--tier", "standard", "--mode", "control-plane",
		"--regions", "us-east-1", "--dry-run"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"kind":"container.orchestration"`)
	require.Contains(t, stdout.String(), `"tier":"standard"`)
	require.Contains(t, stdout.String(), `"mode":"control-plane"`)
}

func TestCompareRejectsTierOnComputeVM(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "compute.vm",
		"--vcpu", "2", "--memory", "8", "--tier", "standard"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, stderr.String(), "kind-flag-mismatch")
}

// M-δ per-kind volume flag tests

func TestCompareVolumeFlag_ops_setsField(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "compute.vm", "--vcpu", "2", "--ops", "1000", "--dry-run"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"ops":1000`)
}

func TestCompareVolumeFlag_queries_setsField(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "compute.vm", "--vcpu", "2", "--queries", "500", "--dry-run"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"queries":500`)
}

func TestCompareVolumeFlag_requests_setsField(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "compute.vm", "--vcpu", "2", "--requests", "200", "--dry-run"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"requests":200`)
}

func TestCompareVolumeFlag_gb_setsField(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "compute.vm", "--vcpu", "2", "--gb", "100.5", "--dry-run"})
	require.NoError(t, cmd.Execute(), stderr.String())
	require.Contains(t, stdout.String(), `"gb":100.5`)
}

func TestCompareCmd_messagingRejectsEngine(t *testing.T) {
	for _, kind := range []string{"messaging.queue", "messaging.topic", "dns.zone"} {
		t.Run(kind, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := newRootCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"compare", "--kind", kind, "--engine", "redis", "--dry-run"})
			err := cmd.Execute()
			require.Error(t, err, "expected --engine to be rejected for %s", kind)
			require.Contains(t, stderr.String(), `"reason":"flag_invalid"`)
			require.NotContains(t, stderr.String(), "--engine",
				"help text must not advertise --engine for %s now that it is rejected", kind)
		})
	}
}

func TestCompareVolumeFlag_mutuallyExclusive(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "ops+queries",
			args: []string{"compare", "--kind", "compute.vm", "--ops", "100", "--queries", "50"},
		},
		{
			name: "ops+requests",
			args: []string{"compare", "--kind", "compute.vm", "--ops", "100", "--requests", "50"},
		},
		{
			name: "ops+gb",
			args: []string{"compare", "--kind", "compute.vm", "--ops", "100", "--gb", "50"},
		},
		{
			name: "queries+requests",
			args: []string{"compare", "--kind", "compute.vm", "--queries", "100", "--requests", "50"},
		},
		{
			name: "all-four",
			args: []string{"compare", "--kind", "compute.vm", "--ops", "100", "--queries", "50", "--requests", "25", "--gb", "10"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := newRootCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			require.Error(t, err, "expected exit code 4 for %s", tc.name)
			require.Contains(t, stderr.String(), `"reason":"flag_invalid"`)
			require.Contains(t, stderr.String(), "volume-flags")
		})
	}
}
