package compare

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func buildShard(t *testing.T, dir, name, seedRel string) string {
	t.Helper()
	seed, err := os.ReadFile(seedRel)
	require.NoError(t, err)
	path := filepath.Join(dir, name+".db")
	require.NoError(t, catalog.BuildFromSQL(path, string(seed)))
	return path
}

func TestRun_mergesSortedByMinPrice(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dir)
	awsPath := buildShard(t, dir, "aws-ec2", filepath.Join("..", "catalog", "testdata", "seed_search.sql"))

	rows, err := Run(context.Background(), Request{
		Kind: "compute.vm",
		VCPU: 2, MemoryGB: 4,
		Regions: []string{"us-east-1", "us-west-2"},
		Sort:    "price",
		Limit:   4,
		Targets: []ShardTarget{{Name: "aws-ec2", Path: awsPath}},
	})
	require.NoError(t, err)
	require.LessOrEqual(t, len(rows), 4)
	for i := 1; i < len(rows); i++ {
		require.GreaterOrEqual(t, rows[i].MinPrice, rows[i-1].MinPrice)
	}
}

func TestRun_cancelledContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dir)
	awsPath := buildShard(t, dir, "aws-ec2", filepath.Join("..", "catalog", "testdata", "seed_search.sql"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Run(ctx, Request{
		Kind: "compute.vm", VCPU: 2,
		Targets: []ShardTarget{{Name: "aws-ec2", Path: awsPath}},
	})
	require.Error(t, err)
}

func TestRun_unsupportedKind(t *testing.T) {
	_, err := Run(context.Background(), Request{Kind: "storage.object"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "storage.object")
}
