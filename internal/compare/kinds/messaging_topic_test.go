package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedMessagingTopicShard(t *testing.T, relPath string) *catalog.Catalog {
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

func TestQueryMessagingTopic_NoFilterReturnsAll(t *testing.T) {
	cat := seedMessagingTopicShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_messaging_topic_compare.sql"))
	rows, err := QueryMessagingTopic(context.Background(), cat, MessagingTopicSpec{})
	require.NoError(t, err)
	require.Len(t, rows, 2, "both standard and premium rows should be returned with no mode filter")
}

func TestQueryMessagingTopic_ModeFilterNarrows(t *testing.T) {
	cat := seedMessagingTopicShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_messaging_topic_compare.sql"))
	rows, err := QueryMessagingTopic(context.Background(), cat, MessagingTopicSpec{
		Mode: "premium",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "mode=premium should return only the premium row")
	require.Equal(t, "premium", rows[0].ResourceAttrs.Extra["mode"])
}

func TestQueryMessagingTopic_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedMessagingTopicShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_messaging_topic_compare.sql"))
	rows, err := QueryMessagingTopic(context.Background(), cat, MessagingTopicSpec{
		Mode: "nonexistent",
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestQueryMessagingTopic_RegionFilterNarrows(t *testing.T) {
	cat := seedMessagingTopicShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_messaging_topic_compare.sql"))
	rows, err := QueryMessagingTopic(context.Background(), cat, MessagingTopicSpec{
		Regions: []string{"us-east-1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.Equal(t, "us-east-1", r.Region)
	}
}
