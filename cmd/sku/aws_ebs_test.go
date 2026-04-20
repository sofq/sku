package sku

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAWSEBSPrice_Seeded(t *testing.T) {
	cat := testutilSeededAWSCatalogM3a2(t)
	t.Setenv("SKU_DATA_DIR", cat.dataDir)

	var out, errOut bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{
		"aws", "ebs", "price",
		"--volume-type", "gp3",
		"--region", "us-east-1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("exec failed: %v stderr=%s", err, errOut.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out.String())
	}
	if doc["resource"].(map[string]any)["name"] != "gp3" {
		t.Fatalf("unexpected row: %s", out.String())
	}
	prices := doc["price"].([]any)
	if len(prices) != 1 || prices[0].(map[string]any)["dimension"] != "storage" {
		t.Fatalf("want single storage price dim, got %v", prices)
	}
}

func TestAWSEBSPrice_NotFoundReturnsError(t *testing.T) {
	cat := testutilSeededAWSCatalogM3a2(t)
	t.Setenv("SKU_DATA_DIR", cat.dataDir)

	root := newRootCmd()
	root.SetErr(&bytes.Buffer{})
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"aws", "ebs", "price", "--volume-type", "nosuch", "--region", "us-east-1"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected not_found error")
	}
}
