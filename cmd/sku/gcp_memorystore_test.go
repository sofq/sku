package sku

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPMemorystorePriceCmd_RequiresInstanceType(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "memorystore", "price", "--region", "us-east1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "instance-type")
}

func TestGCPMemorystorePriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"gcp", "memorystore", "price",
		"--instance-type", "memorystore-redis-standard-16gb",
		"--region", "us-east1",
		"--engine", "redis",
		"--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"gcp memorystore price"`)
	require.Contains(t, buf.String(), `"shards":["gcp-memorystore"]`)
}

func TestGCPMemorystorePrice_HappyPath(t *testing.T) {
	testutilSeededGCPMemorystoreCatalog(t)

	out, _, code := runAzure(t, "gcp", "memorystore", "price",
		"--instance-type", "memorystore-redis-standard-16gb",
		"--region", "us-east1",
		"--engine", "redis",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "gcp", env["provider"])
	require.Equal(t, "memorystore", env["service"])
	require.Contains(t, out, "0.21")
}

func TestGCPMemorystorePrice_NotFound(t *testing.T) {
	testutilSeededGCPMemorystoreCatalog(t)

	_, stderr, code := runAzure(t, "gcp", "memorystore", "price",
		"--instance-type", "memorystore-redis-standard-16gb",
		"--region", "europe-west1",
		"--engine", "redis",
	)
	require.NotZero(t, code)
	require.Contains(t, stderr, "not_found")
}
