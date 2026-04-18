package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPCloudSQLPrice_HappyPath(t *testing.T) {
	testutilSeededGCPCatalogM3b3(t)

	out, _, code := runAzure(t, "gcp", "cloud-sql", "price",
		"--tier", "db-custom-2-7680",
		"--region", "us-east1",
		"--engine", "postgres",
		"--deployment-option", "zonal",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "gcp", env["provider"])
	require.Equal(t, "cloud-sql", env["service"])
	require.Contains(t, out, "0.115")
}

func TestGCPCloudSQLPrice_RegionalCostsMore(t *testing.T) {
	testutilSeededGCPCatalogM3b3(t)

	outZonal, _, _ := runAzure(t, "gcp", "cloud-sql", "price",
		"--tier", "db-custom-2-7680",
		"--region", "us-east1",
		"--engine", "postgres",
		"--deployment-option", "zonal",
	)
	outRegional, _, _ := runAzure(t, "gcp", "cloud-sql", "price",
		"--tier", "db-custom-2-7680",
		"--region", "us-east1",
		"--engine", "postgres",
		"--deployment-option", "regional",
	)
	require.Contains(t, outZonal, "0.115")
	require.Contains(t, outRegional, "0.23")
}

func TestGCPCloudSQLPrice_RequiresTier(t *testing.T) {
	testutilSeededGCPCatalogM3b3(t)

	_, stderr, code := runAzure(t, "gcp", "cloud-sql", "price",
		"--region", "us-east1", "--engine", "postgres",
	)
	require.NotZero(t, code)
	require.Contains(t, stderr, "tier")
}

func TestGCPCloudSQLPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "gcp", "cloud-sql", "price",
		"--tier", "db-custom-2-7680",
		"--region", "us-east1",
		"--engine", "postgres",
		"--deployment-option", "zonal",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"gcp cloud-sql price"`)
	require.Contains(t, out, "gcp-cloud-sql")
}
