package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAWSOpenSearchPriceCmd_RequiresInstanceTypeForManagedCluster(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"aws", "opensearch", "price", "--region", "us-east-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "instance-type")
}

func TestAWSOpenSearchPriceCmd_ServerlessBypassesInstanceTypeRequirement(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"aws", "opensearch", "price", "--mode", "serverless", "--region", "us-east-1", "--dry-run"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	require.NoError(t, cmd.Execute(), buf.String())
	require.Contains(t, buf.String(), `"mode":"serverless"`)
}

func TestAWSOpenSearchPriceCmd_InvalidModeReturnsError(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"aws", "opensearch", "price", "--mode", "invalid", "--instance-type", "r6g.large.search", "--region", "us-east-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "flag_invalid")
}

func TestAWSOpenSearchPriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"aws", "opensearch", "price",
		"--instance-type", "r6g.large.search", "--region", "us-east-1", "--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"aws opensearch price"`)
	require.Contains(t, buf.String(), `"shards":["aws-opensearch"]`)
	require.Contains(t, buf.String(), `"instance_type":"r6g.large.search"`)
}
