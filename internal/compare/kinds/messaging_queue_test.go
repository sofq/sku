package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedMessagingQueueShard(t *testing.T, relPath string) *catalog.Catalog {
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

func TestQueryMessagingQueue_NoFilterReturnsAll(t *testing.T) {
	cat := seedMessagingQueueShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_messaging_queue_compare.sql"))
	rows, err := QueryMessagingQueue(context.Background(), cat, MessagingQueueSpec{})
	require.NoError(t, err)
	require.Len(t, rows, 2, "both standard and fifo rows should be returned with no mode filter")
}

func TestQueryMessagingQueue_ModeFilterNarrows(t *testing.T) {
	cat := seedMessagingQueueShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_messaging_queue_compare.sql"))
	rows, err := QueryMessagingQueue(context.Background(), cat, MessagingQueueSpec{
		Mode: "fifo",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "mode=fifo should return only the fifo row")
	require.Equal(t, "fifo", rows[0].ResourceAttrs.Extra["mode"])
}

func TestQueryMessagingQueue_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedMessagingQueueShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_messaging_queue_compare.sql"))
	rows, err := QueryMessagingQueue(context.Background(), cat, MessagingQueueSpec{
		Mode: "nonexistent",
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestQueryMessagingQueue_RegionFilterNarrows(t *testing.T) {
	cat := seedMessagingQueueShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_messaging_queue_compare.sql"))
	rows, err := QueryMessagingQueue(context.Background(), cat, MessagingQueueSpec{
		Regions: []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "us-east-1", r.Region)
	}
}
