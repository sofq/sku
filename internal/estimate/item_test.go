package estimate

import (
	"strings"
	"testing"
)

func TestParseItem_minimal(t *testing.T) {
	it, err := ParseItem("aws/ec2:m5.large:region=us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if it.Provider != "aws" || it.Service != "ec2" || it.Resource != "m5.large" {
		t.Fatalf("bad provider/service/resource: %+v", it)
	}
	if it.Kind != "compute.vm" {
		t.Fatalf("kind = %q, want compute.vm", it.Kind)
	}
	if it.Params["region"] != "us-east-1" {
		t.Fatalf("region param missing: %+v", it.Params)
	}
}

func TestParseItem_paramsLowercased(t *testing.T) {
	it, err := ParseItem("aws/ec2:m5.large:REGION=us-east-1:COUNT=10:HOURS=730")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range []string{"region", "count", "hours"} {
		if _, ok := it.Params[k]; !ok {
			t.Fatalf("param %q missing after lowercase: %+v", k, it.Params)
		}
	}
}

func TestParseItem_resourcePreservesCase(t *testing.T) {
	it, err := ParseItem("azure/vm:Standard_D2_v3:region=eastus")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if it.Resource != "Standard_D2_v3" {
		t.Fatalf("resource mangled: %q", it.Resource)
	}
}

func TestParseItem_storageObjectKinds(t *testing.T) {
	for _, raw := range []string{
		"aws/s3:standard:region=us-east-1",
		"azure/blob:hot:region=eastus",
		"gcp/gcs:standard:region=us-east1",
	} {
		it, err := ParseItem(raw)
		if err != nil {
			t.Fatalf("%s: parse: %v", raw, err)
		}
		if it.Kind != "storage.object" {
			t.Fatalf("%s: kind = %q, want storage.object", raw, it.Kind)
		}
	}
}

func TestParseItem_errors(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"no slash":         "awsec2:m5.large:region=us-east-1",
		"missing resource": "aws/ec2",
		"empty param key":  "aws/ec2:m5.large:=foo",
		"duplicate key":    "aws/ec2:m5.large:region=us-east-1:region=us-west-2",
		"unsupported pair": "oracle/oci:vm.standard:region=us-ashburn-1",
	}
	for name, raw := range cases {
		if _, err := ParseItem(raw); err == nil {
			t.Errorf("%s: expected error for %q", name, raw)
		} else if !strings.HasPrefix(err.Error(), "estimate/item: ") {
			t.Errorf("%s: error %q lacks estimate/item prefix", name, err.Error())
		}
	}
}
