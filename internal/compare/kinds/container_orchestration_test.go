package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedContainerOrchestrationShard(t *testing.T, relPath string) *catalog.Catalog {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "shard.db")
	data, err := readSQL(relPath)
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(path, data))
	cat, err := catalog.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestQueryContainerOrchestration_DefaultModeReturnsControlPlane(t *testing.T) {
	cat := seedContainerOrchestrationShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_container_orchestration_compare.sql"))
	rows, err := QueryContainerOrchestration(context.Background(), cat, ContainerOrchestrationSpec{
		Regions: []string{"us-east-1", "eastus", "us-east1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "control-plane", r.ResourceAttrs.Extra["mode"])
	}
}

func TestQueryContainerOrchestration_AutopilotMode(t *testing.T) {
	cat := seedContainerOrchestrationShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_container_orchestration_compare.sql"))
	rows, err := QueryContainerOrchestration(context.Background(), cat, ContainerOrchestrationSpec{
		Mode:    "autopilot",
		Regions: []string{"us-east1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "autopilot", r.ResourceAttrs.Extra["mode"])
		require.Equal(t, "gcp", r.Provider)
	}
}

func TestQueryContainerOrchestration_TierFilter(t *testing.T) {
	cat := seedContainerOrchestrationShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_container_orchestration_compare.sql"))
	rows, err := QueryContainerOrchestration(context.Background(), cat, ContainerOrchestrationSpec{
		Tier:    "premium",
		Regions: []string{"eastus"},
	})
	require.NoError(t, err)
	for _, r := range rows {
		require.Equal(t, "azure", r.Provider)
		require.Equal(t, "aks-premium", r.ResourceName)
	}
}

func TestQueryContainerOrchestration_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedContainerOrchestrationShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_container_orchestration_compare.sql"))
	rows, err := QueryContainerOrchestration(context.Background(), cat, ContainerOrchestrationSpec{
		Tier:    "nonexistent",
		Regions: []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}
