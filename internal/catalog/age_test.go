package catalog_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func TestAge_ReturnsDaysSinceGeneratedAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shard.db")
	ddl := `
    CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT);
    INSERT INTO metadata VALUES ('schema_version', '1');
    INSERT INTO metadata VALUES ('catalog_version', '2026.03.29');
    INSERT INTO metadata VALUES ('currency', 'USD');
    INSERT INTO metadata VALUES ('generated_at', '2026-03-29T00:00:00Z');
    CREATE TABLE sku (sku_id TEXT PRIMARY KEY);
    `
	require.NoError(t, catalog.BuildFromSQL(path, ddl))

	cat, err := catalog.Open(path)
	require.NoError(t, err)
	defer func() { _ = cat.Close() }()

	age := cat.Age(time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC))
	require.Equal(t, 20, age)
}
