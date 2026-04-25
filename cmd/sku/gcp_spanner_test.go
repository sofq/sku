package sku

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPSpannerPrice_HappyPath(t *testing.T) {
	testutilSeededGCPSpannerCatalog(t)

	out, _, code := runAzure(t, "gcp", "spanner", "price",
		"--edition", "standard",
		"--region", "us-east1",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "gcp", env["provider"])
	require.Equal(t, "spanner", env["service"])
	require.Contains(t, out, "0.00009")
}

func TestGCPSpannerPrice_NotFound(t *testing.T) {
	testutilSeededGCPSpannerCatalog(t)

	_, stderr, code := runAzure(t, "gcp", "spanner", "price",
		"--edition", "standard",
		"--region", "europe-west1",
	)
	require.NotZero(t, code)
	require.Contains(t, stderr, "not_found")
}

func TestGCPSpannerPriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"gcp", "spanner", "price",
		"--edition", "standard",
		"--region", "us-east1",
		"--pu", "1000",
		"--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"gcp spanner price"`)
	require.Contains(t, buf.String(), `"shards":["gcp-spanner"]`)
}

func TestGCPSpannerPriceCmd_RequiresRegion(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"gcp", "spanner", "price",
		"--edition", "standard",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "region")
}

func TestGCPSpannerPriceCmd_InvalidEdition(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"gcp", "spanner", "price",
		"--edition", "bogus",
		"--region", "us-east1",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "edition")
}
