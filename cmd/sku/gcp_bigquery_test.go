package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPBigQueryPriceCmd_RequiresMode(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "price", "--region", "bq-us"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "mode")
}

func TestGCPBigQueryPriceCmd_RequiresRegion(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "price", "--mode", "on-demand"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "region")
}

func TestGCPBigQueryPriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"gcp", "bigquery", "price",
		"--mode", "on-demand", "--region", "bq-us", "--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"gcp bigquery price"`)
	require.Contains(t, buf.String(), `"shards":["gcp-bigquery"]`)
	require.Contains(t, buf.String(), `"mode":"on-demand"`)
}

func TestGCPBigQueryListCmd_RequiresMode(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "mode")
}
