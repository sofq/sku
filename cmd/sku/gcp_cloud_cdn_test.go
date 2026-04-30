package sku

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPCloudCDNPrice_RequiresRegion(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "cloud-cdn", "price", "--mode", "edge-egress"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "region")
}

func TestGCPCloudCDNPrice_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"gcp", "cloud-cdn", "price",
		"--mode", "edge-egress",
		"--region", "us-east1",
		"--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"gcp cloud-cdn price"`)
	require.Contains(t, buf.String(), `"shards":["gcp-cloud-cdn"]`)
}

func TestGCPCloudCDNPrice_HappyPath(t *testing.T) {
	testutilSeededGCPCloudCDNCatalog(t)

	out, _, code := runAzure(t, "gcp", "cloud-cdn", "price",
		"--mode", "edge-egress",
		"--region", "us-east1",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "gcp", env["provider"])
	require.Equal(t, "cloud-cdn", env["service"])
	resource, ok := env["resource"].(map[string]any)
	require.True(t, ok, "expected resource object in output")
	require.Equal(t, "standard", resource["name"])
}

func TestGCPCloudCDNPrice_NotFound(t *testing.T) {
	testutilSeededGCPCloudCDNCatalog(t)

	_, stderr, code := runAzure(t, "gcp", "cloud-cdn", "price",
		"--mode", "edge-egress",
		"--region", "asia-southeast1",
	)
	require.NotZero(t, code)
	require.Contains(t, stderr, "not_found")
}

func TestGCPCloudCDNList_NoRegion(t *testing.T) {
	testutilSeededGCPCloudCDNCatalog(t)

	out, _, code := runAzure(t, "gcp", "cloud-cdn", "list")
	require.Zero(t, code)
	lines := splitLines(out)
	// Expect 3 rows: 2 egress + 1 request
	require.GreaterOrEqual(t, len(lines), 2)
	for _, line := range lines {
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		require.Equal(t, "gcp", row["provider"])
		require.Equal(t, "cloud-cdn", row["service"])
	}
}

func TestGCPCloudCDNRequest_HappyPath(t *testing.T) {
	testutilSeededGCPCloudCDNCatalog(t)

	out, _, code := runAzure(t, "gcp", "cloud-cdn", "price",
		"--mode", "request",
		"--region", "global",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	location, ok := env["location"].(map[string]any)
	require.True(t, ok, "expected location object in output")
	require.Equal(t, "global", location["provider_region"])
}
