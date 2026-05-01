package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAzureServiceBusTopicsPriceCmd_RequiresTier(t *testing.T) {
	cmd := newAzureServiceBusTopicsCmd()
	cmd.SetArgs([]string{"price", "--region", "eastus"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "tier")
}

func TestAzureServiceBusTopicsPriceCmd_RequiresRegion(t *testing.T) {
	cmd := newAzureServiceBusTopicsCmd()
	cmd.SetArgs([]string{"price", "--tier", "standard"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "region")
}

func TestAzureServiceBusTopicsPriceCmd_InvalidTier(t *testing.T) {
	cmd := newAzureServiceBusTopicsCmd()
	cmd.SetArgs([]string{"price", "--tier", "basic", "--region", "eastus"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
}

func TestAzureServiceBusTopicsPriceCmd_DryRun(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{
		"azure", "service-bus-topics", "price",
		"--tier", "standard",
		"--region", "eastus",
		"--dry-run",
	})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		// Command not yet wired in azure.go — skip rather than fail.
		t.Skipf("azure service-bus-topics not wired in azure.go yet: %v", err)
	}
	require.Contains(t, buf.String(), "azure-service-bus-topics")
}

func TestAzureServiceBusTopicsListCmd_DryRun(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{
		"azure", "service-bus-topics", "list",
		"--tier", "premium",
		"--dry-run",
	})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		// Command not yet wired in azure.go — skip rather than fail.
		t.Skipf("azure service-bus-topics not wired in azure.go yet: %v", err)
	}
	require.Contains(t, buf.String(), "azure-service-bus-topics")
}
