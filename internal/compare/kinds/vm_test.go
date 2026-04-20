package kinds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedAWSEC2(t *testing.T) *catalog.Catalog {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dir)
	seed, err := os.ReadFile(filepath.Join("..", "..", "..", "internal", "catalog", "testdata", "seed_search.sql"))
	require.NoError(t, err)
	path := filepath.Join(dir, "aws-ec2.db")
	require.NoError(t, catalog.BuildFromSQL(path, string(seed)))
	cat, err := catalog.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestQueryVM_minVCPUAndMemory(t *testing.T) {
	cat := seedAWSEC2(t)
	rows, err := QueryVM(context.Background(), cat, VMSpec{VCPU: 4, MemoryGB: 16, Regions: []string{"us-east-1"}})
	require.NoError(t, err)
	for _, r := range rows {
		require.Equal(t, "compute.vm", r.Kind)
		require.NotNil(t, r.ResourceAttrs.VCPU)
		require.GreaterOrEqual(t, *r.ResourceAttrs.VCPU, int64(4))
		require.NotNil(t, r.ResourceAttrs.MemoryGB)
		require.GreaterOrEqual(t, *r.ResourceAttrs.MemoryGB, 16.0)
		require.Equal(t, "us-east-1", r.Region)
	}
	require.NotEmpty(t, rows)
}

func TestQueryVM_excludesGPUByDefault(t *testing.T) {
	cat := seedAWSEC2(t)
	rows, err := QueryVM(context.Background(), cat, VMSpec{VCPU: 2, MemoryGB: 4, Regions: []string{"us-east-1", "us-west-2"}})
	require.NoError(t, err)
	for _, r := range rows {
		if r.ResourceAttrs.GPUCount != nil {
			require.Equal(t, int64(0), *r.ResourceAttrs.GPUCount, "non-GPU compare must not return GPU SKUs")
		}
	}
}

func TestQueryVM_excludesDBRows(t *testing.T) {
	cat := seedAWSEC2(t)
	rows, err := QueryVM(context.Background(), cat, VMSpec{VCPU: 2, MemoryGB: 4, Regions: []string{"us-east-1"}})
	require.NoError(t, err)
	for _, r := range rows {
		require.NotEqual(t, "db.relational", r.Kind)
	}
}
