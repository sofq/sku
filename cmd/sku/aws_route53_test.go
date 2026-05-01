package sku

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

// seedRoute53TestDataDir creates a SKU_DATA_DIR with an aws-route53.db
// populated from internal/catalog/testdata/seed_dns_zone.sql.
func seedRoute53TestDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_dns_zone.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "aws-route53.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
	return dir
}

func TestAWSRoute53Price_PublicHappyPath(t *testing.T) {
	seedRoute53TestDataDir(t)

	out, _, code := runAWS(t, "aws", "route53", "price",
		"--zone-type", "public",
		"--region", "global",
		"--stale-ok",
	)
	require.Zero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "route53", env["service"])
	require.Equal(t, "aws", env["provider"])
}

func TestAWSRoute53Price_PrivateHappyPath(t *testing.T) {
	seedRoute53TestDataDir(t)

	out, _, code := runAWS(t, "aws", "route53", "price",
		"--zone-type", "private",
		"--region", "global",
		"--stale-ok",
	)
	require.Zero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "route53", env["service"])
}

func TestAWSRoute53Price_NotFound(t *testing.T) {
	seedRoute53TestDataDir(t)

	_, stderr, code := runAWS(t, "aws", "route53", "price",
		"--zone-type", "nosuchtype",
		"--region", "global",
		"--stale-ok",
	)
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
}

func TestAWSRoute53Price_ShardMissing(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())

	_, stderr, code := runAWS(t, "aws", "route53", "price",
		"--zone-type", "public",
		"--region", "global",
	)
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	details := body["details"].(map[string]any)
	require.Equal(t, "aws-route53", details["shard"])
}

func TestAWSRoute53List_ReturnsBothTypes(t *testing.T) {
	seedRoute53TestDataDir(t)

	out, _, code := runAWS(t, "aws", "route53", "list",
		"--zone-type", "public",
		"--stale-ok",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
}

func TestAWSRoute53Price_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())

	out, _, code := runAWS(t, "aws", "route53", "price",
		"--zone-type", "public", "--region", "global", "--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"aws route53 price"`)
}
