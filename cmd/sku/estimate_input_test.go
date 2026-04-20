package sku

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadEstimateStdin_json(t *testing.T) {
	r := strings.NewReader(`{"items":[{"provider":"aws","service":"ec2","resource":"m5.large","params":{"region":"us-east-1"}}]}`)
	items, err := readEstimateStdin(r)
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	if len(items) != 1 || items[0].Resource != "m5.large" {
		t.Fatalf("items: %+v", items)
	}
}

func TestReadEstimateConfig_yamlExt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "w.yaml")
	if err := os.WriteFile(p, []byte("items:\n  - provider: aws\n    service: ec2\n    resource: m5.large\n    params: {region: us-east-1}\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	items, err := readEstimateConfig(p)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
}

func TestReadEstimateConfig_jsonExt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "w.json")
	if err := os.WriteFile(p, []byte(`{"items":[{"provider":"aws","service":"ec2","resource":"m5.large"}]}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readEstimateConfig(p); err != nil {
		t.Fatalf("config: %v", err)
	}
}

func TestReadEstimateConfig_unknownExt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "w.toml")
	if err := os.WriteFile(p, []byte("items = []"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := readEstimateConfig(p)
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
	if !strings.Contains(err.Error(), ".yaml") {
		t.Fatalf("hint missing: %v", err)
	}
}
