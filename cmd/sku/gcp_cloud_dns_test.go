package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPCloudDNSPriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"gcp", "cloud-dns", "price",
		"--zone-type", "public",
		"--region", "global",
		"--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"gcp cloud-dns price"`)
	require.Contains(t, buf.String(), `"shards":["gcp-cloud-dns"]`)
}

func TestGCPCloudDNSListCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"gcp", "cloud-dns", "list",
		"--zone-type", "public",
		"--region", "global",
		"--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"gcp cloud-dns list"`)
	require.Contains(t, buf.String(), `"shards":["gcp-cloud-dns"]`)
}

func TestGCPCloudDNSPrice_ShardMissing(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())

	_, stderr, code := runAzure(t, "gcp", "cloud-dns", "price",
		"--zone-type", "public",
		"--region", "global",
	)
	require.NotZero(t, code)
	require.Contains(t, stderr, "gcp-cloud-dns")
}
