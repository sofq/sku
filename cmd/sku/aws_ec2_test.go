package sku

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

// seedAWSTestDataDir creates a SKU_DATA_DIR containing both aws-ec2.db and
// aws-rds.db populated from internal/catalog/testdata/seed_aws.sql.
func seedAWSTestDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_aws.sql"))
	require.NoError(t, err)
	for _, shard := range []string{"aws-ec2", "aws-rds"} {
		require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, shard+".db"), string(ddl)))
	}
	t.Setenv("SKU_DATA_DIR", dir)
	return dir
}

func runAWS(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return out.String(), errb.String(), 1
	}
	return out.String(), errb.String(), 0
}

func TestAWSEC2Price_HappyPath(t *testing.T) {
	seedAWSTestDataDir(t)

	out, _, code := runAWS(t, "aws", "ec2", "price",
		"--instance-type", "m5.large",
		"--region", "us-east-1",
		"--os", "linux", "--tenancy", "shared",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "ec2", env["service"])
	require.Equal(t, "aws", env["provider"])
	resource := env["resource"].(map[string]any)
	require.Equal(t, "m5.large", resource["name"])
	location := env["location"].(map[string]any)
	require.Equal(t, "us-east-1", location["provider_region"])
}

func TestAWSEC2Price_NotFound_ReturnsExit3(t *testing.T) {
	seedAWSTestDataDir(t)

	_, stderr, code := runAWS(t, "aws", "ec2", "price",
		"--instance-type", "m5.nosuch",
		"--region", "us-east-1",
	)
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
}

func TestAWSEC2Price_MissingRegion_ReturnsValidation(t *testing.T) {
	seedAWSTestDataDir(t)

	_, stderr, code := runAWS(t, "aws", "ec2", "price", "--instance-type", "m5.large")
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "validation", body["code"])
}

func TestAWSEC2List_ReturnsMultipleRegions(t *testing.T) {
	seedAWSTestDataDir(t)

	out, _, code := runAWS(t, "aws", "ec2", "list", "--instance-type", "m5.large")
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 2)
}

func TestAWSEC2Price_ShardMissing_ReturnsNotFound(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())

	_, stderr, code := runAWS(t, "aws", "ec2", "price",
		"--instance-type", "m5.large",
		"--region", "us-east-1",
	)
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
	details := body["details"].(map[string]any)
	require.Equal(t, "aws-ec2", details["shard"])
}

func TestAWSEC2Price_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAWS(t, "aws", "ec2", "price",
		"--instance-type", "m5.large", "--region", "us-east-1", "--dry-run")
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"aws ec2 price"`)
}
