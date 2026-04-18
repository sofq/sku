package sku

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAWSLambdaPrice_Seeded(t *testing.T) {
	cat := testutilSeededAWSCatalogM3a2(t)
	t.Setenv("SKU_DATA_DIR", cat.dataDir)

	var out, errOut bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{
		"aws", "lambda", "price",
		"--architecture", "arm64",
		"--region", "us-east-1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("exec failed: %v stderr=%s", err, errOut.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out.String())
	}
	if doc["resource"].(map[string]any)["name"] != "arm64" {
		t.Fatalf("unexpected row: %s", out.String())
	}
}

func TestAWSLambdaList_NoRegionReturnsAllRegions(t *testing.T) {
	cat := testutilSeededAWSCatalogM3a2(t)
	t.Setenv("SKU_DATA_DIR", cat.dataDir)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"aws", "lambda", "list", "--architecture", "x86_64"})
	if err := root.Execute(); err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected rows, got empty output")
	}
}
