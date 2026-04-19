package estimate

import (
	"strings"
	"testing"
)

func TestDecodeWorkload_jsonHappyPath(t *testing.T) {
	in := strings.NewReader(`{
	  "items": [
	    {"provider":"aws","service":"ec2","resource":"m5.large",
	     "params":{"region":"us-east-1","count":"2","hours":"100"}}
	  ]
	}`)
	items, err := DecodeWorkload(in, "json")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	got := items[0]
	if got.Provider != "aws" || got.Service != "ec2" || got.Resource != "m5.large" {
		t.Fatalf("bad item: %+v", got)
	}
	if got.Kind != "compute.vm" {
		t.Fatalf("kind = %q, want compute.vm", got.Kind)
	}
	if got.Params["region"] != "us-east-1" || got.Params["count"] != "2" {
		t.Fatalf("params: %+v", got.Params)
	}
	if got.Raw != "aws/ec2:m5.large:count=2:hours=100:region=us-east-1" {
		t.Fatalf("raw = %q", got.Raw)
	}
}

func TestDecodeWorkload_yamlHappyPath(t *testing.T) {
	in := strings.NewReader(`items:
  - provider: aws
    service: ec2
    resource: m5.large
    params:
      region: us-east-1
      count: 3
      hours: 730
`)
	items, err := DecodeWorkload(in, "yaml")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Params["count"] != "3" {
		t.Fatalf("count = %q, want 3", items[0].Params["count"])
	}
	if items[0].Params["hours"] != "730" {
		t.Fatalf("hours = %q, want 730", items[0].Params["hours"])
	}
}

func TestDecodeWorkload_rejectsEmptyItems(t *testing.T) {
	in := strings.NewReader(`{"items":[]}`)
	if _, err := DecodeWorkload(in, "json"); err == nil {
		t.Fatal("expected error for empty items")
	}
}

func TestDecodeWorkload_rejectsUnknownField(t *testing.T) {
	in := strings.NewReader(`{"items":[{"provider":"aws","service":"ec2","resource":"m5.large","extra":1}]}`)
	if _, err := DecodeWorkload(in, "json"); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestDecodeWorkload_rejectsUnknownProviderService(t *testing.T) {
	in := strings.NewReader(`{"items":[{"provider":"acme","service":"widgets","resource":"x"}]}`)
	_, err := DecodeWorkload(in, "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported provider/service") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestDecodeWorkload_rejectsOversizeInput(t *testing.T) {
	big := strings.Repeat("a", (1<<20)+10)
	_, err := DecodeWorkload(strings.NewReader(big), "json")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestDecodeWorkload_llmText(t *testing.T) {
	yamlIn := `items:
  - provider: llm
    service: text
    resource: anthropic/claude-opus-4.6
    params:
      input: 1M
      output: 500K
`
	items, err := DecodeWorkload(strings.NewReader(yamlIn), "yaml")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	it := items[0]
	if it.Kind != "llm.text" {
		t.Fatalf("kind = %q, want llm.text", it.Kind)
	}
	if it.Resource != "anthropic/claude-opus-4.6" {
		t.Fatalf("resource = %q", it.Resource)
	}
	if it.Params["input"] != "1M" || it.Params["output"] != "500K" {
		t.Fatalf("params = %v", it.Params)
	}
}
