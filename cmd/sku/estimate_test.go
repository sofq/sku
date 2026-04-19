package sku

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEstimate_endToEnd(t *testing.T) {
	_ = testutilSeededEstimateCatalog(t)

	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"estimate",
		"--item", "aws/ec2:m5.large:region=us-east-1:count=2:hours=100",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr: %s", err, stderr.String())
	}
	var obj map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	items, _ := obj["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	total, _ := obj["monthly_total"].(map[string]any)
	if total == nil {
		t.Fatalf("missing monthly_total in %s", stdout.String())
	}
}

func TestEstimate_requiresItem(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"estimate"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when no --item is given")
	}
}

func TestEstimate_stdinJSON(t *testing.T) {
	_ = testutilSeededEstimateCatalog(t)

	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(`{"items":[{"provider":"aws","service":"ec2","resource":"m5.large","params":{"region":"us-east-1","count":"2","hours":"100"}}]}`))
	root.SetArgs([]string{"estimate", "--stdin"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr: %s", err, stderr.String())
	}
	var obj map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	items, _ := obj["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
}

func TestEstimate_configYAML(t *testing.T) {
	_ = testutilSeededEstimateCatalog(t)

	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"estimate", "--config", "testdata/workload-vm.yaml"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr: %s", err, stderr.String())
	}
	var obj map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	total, _ := obj["monthly_total"].(map[string]any)
	if total == nil {
		t.Fatalf("missing monthly_total in %s", stdout.String())
	}
}

func TestEstimate_mutuallyExclusive(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"estimate",
		"--item", "aws/ec2:m5.large:region=us-east-1",
		"--stdin",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when both --item and --stdin are set")
	}
	if !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Fatalf("stderr missing hint: %s", stderr.String())
	}
}
