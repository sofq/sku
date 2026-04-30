package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureAppServicePriceCmd_RequiresSKU(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"azure", "appservice", "price", "--region", "eastus"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "sku")
}

func TestAzureAppServicePriceCmd_InvalidOSReturnsError(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"azure", "appservice", "price", "--sku", "P1v3", "--region", "eastus", "--os", "freebsd"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "flag_invalid")
}

func TestAzureAppServicePriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"azure", "appservice", "price",
		"--sku", "P1v3", "--region", "eastus", "--os", "linux", "--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"azure appservice price"`)
	require.Contains(t, buf.String(), `"shards":["azure-appservice"]`)
	require.Contains(t, buf.String(), `"sku":"P1v3"`)
}

func TestAzureAppServicePriceCmd_DefaultsOSToLinux(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"azure", "appservice", "price",
		"--sku", "P1v3", "--region", "eastus", "--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"os":"linux"`)
}
