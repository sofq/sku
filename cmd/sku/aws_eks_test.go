package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAWSEKSPriceCmd_RequiresTierForControlPlane(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"aws", "eks", "price", "--region", "us-east-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "tier")
}

func TestAWSEKSPriceCmd_FargateSkipsTierRequirement(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"aws", "eks", "price", "--mode", "fargate", "--region", "us-east-1", "--dry-run"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	require.NoError(t, cmd.Execute(), buf.String())
	require.Contains(t, buf.String(), `"mode":"fargate"`)
}

func TestAWSEKSPriceCmd_InvalidModeReturnsError(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"aws", "eks", "price", "--mode", "invalid", "--tier", "standard", "--region", "us-east-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "flag_invalid")
}

func TestAWSEKSPriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"aws", "eks", "price",
		"--tier", "standard", "--region", "us-east-1", "--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"aws eks price"`)
	require.Contains(t, buf.String(), `"shards":["aws-eks"]`)
	require.Contains(t, buf.String(), `"tier":"standard"`)
}
