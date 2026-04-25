package sku

import (
	"bytes"
	"strings"
	"testing"
)

func TestAWSDynamoDB_Price_SeededUSE1(t *testing.T) {
	testutilSeededAWSCatalogM3a3(t)

	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "dynamodb", "price",
		"--table-class", "standard", "--region", "us-east-1"})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr=%s", err, errb.String())
	}
	if !strings.Contains(out.String(), `"dimension":"read_request_units"`) {
		t.Fatalf("stdout missing read_request_units dim: %s", out.String())
	}
}

func TestAWSDynamoDB_Price_MissingRequiredFlag(t *testing.T) {
	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "dynamodb", "price", "--region", "us-east-1"})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	err := rc.Execute()
	if err == nil {
		t.Fatal("want error for missing --table-class")
	}
}

func TestAWSDynamoDB_List_DropsRegion(t *testing.T) {
	testutilSeededAWSCatalogM3a3(t)
	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "dynamodb", "list", "--table-class", "standard"})
	var out bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&bytes.Buffer{})
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"us-east-1"`) {
		t.Fatalf("stdout missing us-east-1: %s", out.String())
	}
}
