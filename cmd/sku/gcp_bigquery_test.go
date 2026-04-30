package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPBigQueryPriceCmd_DefaultsModeToOnDemand(t *testing.T) {
	// Mode defaults to on-demand to mirror `compare --kind warehouse.query`.
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "price", "--region", "bq-us", "--dry-run"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	require.NoError(t, cmd.Execute(), buf.String())
	require.Contains(t, buf.String(), `"mode":"on-demand"`)
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

func TestGCPBigQueryListCmd_DefaultsModeToOnDemand(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "list", "--dry-run"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	require.NoError(t, cmd.Execute(), buf.String())
	require.Contains(t, buf.String(), `"mode":"on-demand"`)
}

func TestGCPBigQueryPriceCmd_RejectsUnknownMode(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "price", "--mode", "bogus", "--region", "bq-us"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "flag_invalid")
}

// Real-lookup tests below open a seeded gcp-bigquery shard and exercise the
// catalog query path end-to-end. These guard against the class of bug where
// the CLI's Terms tuple disagrees with what the ingest writes.

func TestGCPBigQueryPrice_OnDemand_HappyPath(t *testing.T) {
	testutilSeededGCPBigQueryCatalog(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "price", "--mode", "on-demand", "--region", "bq-us"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	require.NoError(t, cmd.Execute(), errb.String())
	require.Contains(t, out.String(), `"name":"on-demand"`)
}

func TestGCPBigQueryPrice_CapacityStandard_HappyPath(t *testing.T) {
	testutilSeededGCPBigQueryCatalog(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "price", "--mode", "capacity-standard", "--region", "bq-us"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	require.NoError(t, cmd.Execute(), errb.String())
	require.Contains(t, out.String(), `"name":"capacity-standard"`)
}

func TestGCPBigQueryPrice_CapacityEnterprisePlus_HappyPath(t *testing.T) {
	testutilSeededGCPBigQueryCatalog(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "price", "--mode", "capacity-enterprise-plus", "--region", "bq-us"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	require.NoError(t, cmd.Execute(), errb.String())
	require.Contains(t, out.String(), `"name":"capacity-enterprise-plus"`)
}

func TestGCPBigQueryList_DropsRegion(t *testing.T) {
	testutilSeededGCPBigQueryCatalog(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "bigquery", "list", "--mode", "storage-active"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	require.NoError(t, cmd.Execute(), errb.String())
	require.Contains(t, out.String(), `"name":"storage-active"`)
}
