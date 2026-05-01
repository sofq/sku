package kinds

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedAPIGatewayShard(t *testing.T, relPath string) *catalog.Catalog {
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

func TestQueryAPIGateway_NoFilterReturnsAll(t *testing.T) {
	cat := seedAPIGatewayShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_api_gateway_compare.sql"))
	rows, err := QueryAPIGateway(context.Background(), cat, APIGatewaySpec{})
	require.NoError(t, err)
	require.Len(t, rows, 3, "all three rows (rest, http, provisioned) should be returned with no mode filter")
}

func TestQueryAPIGateway_ModeFilterNarrows(t *testing.T) {
	cat := seedAPIGatewayShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_api_gateway_compare.sql"))
	rows, err := QueryAPIGateway(context.Background(), cat, APIGatewaySpec{
		Mode: "http",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "mode=http should return only the http row")
	require.Equal(t, "http", rows[0].ResourceAttrs.Extra["mode"])
}

func TestQueryAPIGateway_NoMatchReturnsEmpty(t *testing.T) {
	cat := seedAPIGatewayShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_api_gateway_compare.sql"))
	rows, err := QueryAPIGateway(context.Background(), cat, APIGatewaySpec{
		Mode: "nonexistent",
	})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestQueryAPIGateway_MixedUnitWarnsWhenNoMode(t *testing.T) {
	cat := seedAPIGatewayShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_api_gateway_compare.sql"))

	var buf bytes.Buffer
	prev := SetWarningWriter(&buf)
	t.Cleanup(func() { SetWarningWriter(prev) })

	// No mode filter: result mixes "1M-req" (rest/http) and "hr" (provisioned).
	rows, err := QueryAPIGateway(context.Background(), cat, APIGatewaySpec{})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Contains(t, buf.String(), "api.gateway result mixes per-call and per-unit-hour pricing")
}

func TestQueryAPIGateway_MixedUnitNoWarnWithMode(t *testing.T) {
	cat := seedAPIGatewayShard(t, filepath.Join("..", "..", "catalog", "testdata", "seed_api_gateway_compare.sql"))

	var buf bytes.Buffer
	prev := SetWarningWriter(&buf)
	t.Cleanup(func() { SetWarningWriter(prev) })

	// Mode filter set: no mixed-unit warning should be emitted.
	rows, err := QueryAPIGateway(context.Background(), cat, APIGatewaySpec{Mode: "rest"})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.NotContains(t, buf.String(), "api.gateway result mixes")
}
