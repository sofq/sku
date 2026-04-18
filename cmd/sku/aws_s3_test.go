package sku

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAWSS3Price_Seeded(t *testing.T) {
	cat := testutilSeededAWSCatalogM3a2(t)
	t.Setenv("SKU_DATA_DIR", cat.dataDir)

	var out, errOut bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{
		"aws", "s3", "price",
		"--storage-class", "standard",
		"--region", "us-east-1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("exec failed: %v stderr=%s", err, errOut.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out.String())
	}
	if doc["resource"].(map[string]any)["name"] != "standard" {
		t.Fatalf("unexpected row: %s", out.String())
	}
	if !strings.Contains(out.String(), "requests-get") {
		t.Fatalf("missing requests-get dimension: %s", out.String())
	}
}

func TestAWSS3Price_MissingRegionReturnsValidation(t *testing.T) {
	cat := testutilSeededAWSCatalogM3a2(t)
	t.Setenv("SKU_DATA_DIR", cat.dataDir)

	root := newRootCmd()
	root.SetErr(&bytes.Buffer{})
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"aws", "s3", "price", "--storage-class", "standard"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected validation error for missing --region")
	}
}
