package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAWSAuroraPriceCmd_RequiresInstanceType(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"aws", "aurora", "price", "--region", "us-east-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "instance-type")
}

func TestAWSAuroraPriceCmd_DryRun(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"aws", "aurora", "price",
		"--instance-type", "db.r6g.large",
		"--region", "us-east-1",
		"--engine", "aurora-postgres",
		"--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"aws aurora price"`)
	require.Contains(t, buf.String(), `"shards":["aws-aurora"]`)
}
