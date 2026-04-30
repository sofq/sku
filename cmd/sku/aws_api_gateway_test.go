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

// seededAWSAPIGatewayCatalog holds the temp dir for the api-gateway shard.
type seededAWSAPIGatewayCatalog struct{ dataDir string }

// testutilSeededAWSAPIGatewayCatalog creates a SKU_DATA_DIR containing
// aws-api-gateway.db populated from seed_aws_api_gateway.sql.
func testutilSeededAWSAPIGatewayCatalog(t *testing.T) seededAWSAPIGatewayCatalog {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_aws_api_gateway.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "aws-api-gateway.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
	return seededAWSAPIGatewayCatalog{dataDir: dir}
}

func TestAWSAPIGatewayPrice_Seeded(t *testing.T) {
	cat := testutilSeededAWSAPIGatewayCatalog(t)
	t.Setenv("SKU_DATA_DIR", cat.dataDir)

	var out, errOut bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{
		"aws", "api-gateway", "price",
		"--api-type", "rest",
		"--region", "us-east-1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("exec failed: %v stderr=%s", err, errOut.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out.String())
	}
	if doc["resource"].(map[string]any)["name"] != "rest" {
		t.Fatalf("unexpected resource name: %s", out.String())
	}
}

func TestAWSAPIGatewayList_NoRegionReturnsRows(t *testing.T) {
	cat := testutilSeededAWSAPIGatewayCatalog(t)
	t.Setenv("SKU_DATA_DIR", cat.dataDir)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"aws", "api-gateway", "list", "--api-type", "http"})
	if err := root.Execute(); err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected rows, got empty output")
	}
}

func TestAWSAPIGatewayPrice_DryRun(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"aws", "api-gateway", "price",
		"--api-type", "rest",
		"--region", "us-east-1",
		"--dry-run",
	})
	require.NoError(t, root.Execute())
	require.Contains(t, buf.String(), `"command":"aws api-gateway price"`)
	require.Contains(t, buf.String(), `"shards":["aws-api-gateway"]`)
	require.Contains(t, buf.String(), `"api_type":"rest"`)
}

func TestAWSAPIGatewayPrice_InvalidApiType(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"aws", "api-gateway", "price",
		"--api-type", "websocket",
		"--region", "us-east-1",
	})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "flag_invalid")
}

func TestAWSAPIGatewayPrice_MissingRegion(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"aws", "api-gateway", "price",
		"--api-type", "rest",
	})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "flag_invalid")
}
