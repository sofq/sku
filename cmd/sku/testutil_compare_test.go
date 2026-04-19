package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

// testutilSeededComputeVMCatalog builds aws-ec2, azure-vm, and gcp-gce shards
// from the checked-in m3 fixtures into one temp SKU_DATA_DIR and returns it.
// Each underlying seed ships compute.vm rows across us-east* regions so
// fan-out tests exercise real cross-shard merging.
func testutilSeededComputeVMCatalog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dir)

	pairs := []struct {
		shard   string
		seedRel string
	}{
		{"aws-ec2", filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_aws.sql")},
		{"azure-vm", filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_azure.sql")},
		{"gcp-gce", filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_gcp.sql")},
	}
	for _, p := range pairs {
		seed, err := os.ReadFile(p.seedRel) //nolint:gosec
		require.NoError(t, err)
		require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, p.shard+".db"), string(seed)))
	}
	return dir
}
