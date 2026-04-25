package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureRedisPriceCmd_RequiresSize(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"azure", "redis", "price", "--region", "eastus"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "size")
}

func TestAzureRedisPriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"azure", "redis", "price",
		"--tier", "standard",
		"--size", "C1",
		"--region", "eastus",
		"--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), "azure-redis")
}
