package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAWSElastiCachePriceCmd_RequiresInstanceType(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"aws", "elasticache", "price", "--region", "us-east-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "instance-type")
}

func TestAWSElastiCachePriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"aws", "elasticache", "price",
		"--instance-type", "cache.r6g.large",
		"--region", "us-east-1",
		"--engine", "redis",
		"--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"aws elasticache price"`)
	require.Contains(t, buf.String(), `"shards":["aws-elasticache"]`)
}
