package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

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
