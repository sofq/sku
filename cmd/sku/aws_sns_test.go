package sku

import (
	"bytes"
	"strings"
	"testing"
)

func TestAWSSNS_Price_SeededUSE1(t *testing.T) {
	testutilSeededAWSSNSCatalog(t)

	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "sns", "price", "--region", "us-east-1", "--stale-ok"})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr=%s", err, errb.String())
	}
	if !strings.Contains(out.String(), `"dimension":"request"`) {
		t.Fatalf("stdout missing request dimension: %s", out.String())
	}
}

func TestAWSSNS_Price_MissingRegionFlag(t *testing.T) {
	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "sns", "price"})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	err := rc.Execute()
	if err == nil {
		t.Fatal("want error for missing --region")
	}
}

func TestAWSSNS_List_AllRegions(t *testing.T) {
	testutilSeededAWSSNSCatalog(t)
	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "sns", "list", "--stale-ok"})
	var out bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&bytes.Buffer{})
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"us-east-1"`) {
		t.Fatalf("stdout missing us-east-1: %s", out.String())
	}
	if !strings.Contains(out.String(), `"eu-west-1"`) {
		t.Fatalf("stdout missing eu-west-1: %s", out.String())
	}
}
