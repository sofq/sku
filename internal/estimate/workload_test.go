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
