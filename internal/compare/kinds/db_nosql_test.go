package kinds

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedDBNoSQLShard(t *testing.T, relPath string) *catalog.Catalog {
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

func TestQueryDBNoSQL_NoFilterReturnsAll(t *testing.T) {
	cat := seedDBNoSQLShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_db_nosql_compare.sql"))
	rows, err := QueryDBNoSQL(context.Background(), cat, DBNoSQLSpec{})
	require.NoError(t, err)
	require.Len(t, rows, 3, "all three rows should be returned with no filters")
}

func TestQueryDBNoSQL_EngineFilterNarrows(t *testing.T) {
	cat := seedDBNoSQLShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_db_nosql_compare.sql"))
	rows, err := QueryDBNoSQL(context.Background(), cat, DBNoSQLSpec{
		Engine: "dynamodb",
	})
	require.NoError(t, err)
	require.Len(t, rows, 2, "engine=dynamodb should return both dynamodb rows (provisioned + serverless)")
	for _, r := range rows {
		require.Equal(t, "dynamodb", r.Terms.Tenancy)
	}
}

func TestQueryDBNoSQL_ModeFilterNarrows(t *testing.T) {
	cat := seedDBNoSQLShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_db_nosql_compare.sql"))
	rows, err := QueryDBNoSQL(context.Background(), cat, DBNoSQLSpec{
		Mode: "serverless",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "mode=serverless should return only the serverless row")
	require.Equal(t, "serverless", rows[0].ResourceAttrs.Extra["mode"])
}

func TestQueryDBNoSQL_EngineAndModeFilterNarrows(t *testing.T) {
	cat := seedDBNoSQLShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_db_nosql_compare.sql"))
	rows, err := QueryDBNoSQL(context.Background(), cat, DBNoSQLSpec{
		Engine: "dynamodb",
		Mode:   "provisioned",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "engine=dynamodb + mode=provisioned should return exactly 1 row")
	require.Equal(t, "dynamodb", rows[0].Terms.Tenancy)
	require.Equal(t, "provisioned", rows[0].ResourceAttrs.Extra["mode"])
}

func TestQueryDBNoSQL_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedDBNoSQLShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_db_nosql_compare.sql"))
	rows, err := QueryDBNoSQL(context.Background(), cat, DBNoSQLSpec{
		Engine: "nonexistent",
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}
