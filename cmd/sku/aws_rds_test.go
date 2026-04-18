package sku

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAWSRDSPrice_MultiAZHappyPath(t *testing.T) {
	seedAWSTestDataDir(t)

	out, _, code := runAWS(t, "aws", "rds", "price",
		"--instance-type", "db.m5.large",
		"--region", "us-east-1",
		"--engine", "postgres",
		"--deployment-option", "multi-az",
	)
	require.Zero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "rds", env["service"])
	prices := env["price"].([]any)
	require.Len(t, prices, 1)
	first := prices[0].(map[string]any)
	require.InDelta(t, 0.300, first["amount"], 1e-9)
}

func TestAWSRDSPrice_SingleAZReturnsCheaper(t *testing.T) {
	seedAWSTestDataDir(t)

	out, _, code := runAWS(t, "aws", "rds", "price",
		"--instance-type", "db.m5.large",
		"--region", "us-east-1",
		"--engine", "postgres",
		"--deployment-option", "single-az",
	)
	require.Zero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	prices := env["price"].([]any)
	first := prices[0].(map[string]any)
	require.InDelta(t, 0.150, first["amount"], 1e-9)
}

func TestAWSRDSPrice_NotFound(t *testing.T) {
	seedAWSTestDataDir(t)

	_, stderr, code := runAWS(t, "aws", "rds", "price",
		"--instance-type", "db.m5.xlarge", // not seeded
		"--region", "us-east-1",
		"--engine", "postgres",
		"--deployment-option", "single-az",
	)
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
}

func TestAWSRDSPrice_ShardMissing(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())

	_, stderr, code := runAWS(t, "aws", "rds", "price",
		"--instance-type", "db.m5.large", "--region", "us-east-1",
		"--engine", "postgres", "--deployment-option", "single-az",
	)
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	details := body["details"].(map[string]any)
	require.Equal(t, "aws-rds", details["shard"])
}
