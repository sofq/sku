package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPGKERequiresTierForControlPlane(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "gke", "price", "--mode", "control-plane", "--region", "us-east1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "tier")
}

func TestGCPGKEAutopilotDefaultsTier(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "gke", "price", "--mode", "autopilot", "--region", "us-east1", "--dry-run"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"tier":"autopilot"`)
}

func TestGCPGKEInvalidMode(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gcp", "gke", "price", "--mode", "invalid", "--tier", "standard", "--region", "us-east1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "flag_invalid")
}

func TestGCPGKEDryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"gcp", "gke", "price",
		"--tier", "standard", "--mode", "control-plane", "--region", "us-east1", "--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"gcp gke price"`)
	require.Contains(t, buf.String(), `"shards":["gcp-gke"]`)
	require.Contains(t, buf.String(), `"tier":"standard"`)
}
