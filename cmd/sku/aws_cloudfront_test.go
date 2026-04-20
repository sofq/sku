package sku

import (
	"bytes"
	"strings"
	"testing"
)

func TestAWSCloudFront_Price_SeededEU(t *testing.T) {
	testutilSeededAWSCatalogM3a3(t)

	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "cloudfront", "price",
		"--resource-name", "standard", "--region", "eu-west-1"})
	var out bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&bytes.Buffer{})
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"data_transfer_out"`) {
		t.Fatalf("stdout missing data_transfer_out: %s", out.String())
	}
	if !strings.Contains(out.String(), `"amount":0.085`) {
		t.Fatalf("stdout missing amount 0.085: %s", out.String())
	}
}

func TestAWSCloudFront_Price_DefaultsResourceName(t *testing.T) {
	testutilSeededAWSCatalogM3a3(t)
	rc := newRootCmd()
	// --resource-name defaults to "standard" so the CLI is usable without it.
	rc.SetArgs([]string{"aws", "cloudfront", "price", "--region", "eu-west-1"})
	var out bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&bytes.Buffer{})
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"provider_region":"eu-west-1"`) {
		t.Fatalf("stdout missing provider_region: %s", out.String())
	}
}

func TestAWSCloudFront_Price_MissingRegion(t *testing.T) {
	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "cloudfront", "price"})
	err := rc.Execute()
	if err == nil {
		t.Fatal("want error for missing --region")
	}
}
