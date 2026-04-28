package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureAKSPriceCmd_RequiresTierForControlPlane(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"azure", "aks", "price", "--region", "eastus"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "tier")
}

func TestAzureAKSPriceCmd_VirtualNodesRequiresAciOS(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"azure", "aks", "price", "--mode", "virtual-nodes", "--aci-os", "", "--region", "eastus"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "aci-os")
}

func TestAzureAKSPriceCmd_InvalidModeReturnsError(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"azure", "aks", "price", "--mode", "invalid", "--tier", "standard", "--region", "eastus"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "flag_invalid")
}

func TestAzureAKSPriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"azure", "aks", "price",
		"--tier", "standard", "--region", "eastus", "--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"azure aks price"`)
	require.Contains(t, buf.String(), `"shards":["azure-aks"]`)
	require.Contains(t, buf.String(), `"tier":"standard"`)
}
