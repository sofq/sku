package estimate

import (
	"context"
	"math"
	"testing"

	"github.com/sofq/sku/internal/catalog"
)

func TestStorageObjectEstimator_storageOnly(t *testing.T) {
	resetRegistry(t)
	prev := lookupStorageObject
	t.Cleanup(func() { lookupStorageObject = prev })
	lookupStorageObject = func(_ context.Context, _ string, _ catalog.StorageObjectFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "s3-standard-use1", Provider: "aws", Service: "s3",
			ResourceName: "standard", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "storage", Amount: 0.023, Unit: "gb-mo"}},
		}}, nil
	}

	it, err := ParseItem("aws/s3:standard:region=us-east-1:gb_month=500")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	li, err := storageObjectEstimator{}.Estimate(context.Background(), it)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	want := 0.023 * 500
	if math.Abs(li.MonthlyUSD-want) > 1e-9 {
		t.Fatalf("monthly = %v, want %v", li.MonthlyUSD, want)
	}
	if li.Quantity != 500 || li.QuantityUnit != "gb-mo" {
		t.Fatalf("bad line: %+v", li)
	}
}

func TestStorageObjectEstimator_allDimensions(t *testing.T) {
	prev := lookupStorageObject
	t.Cleanup(func() { lookupStorageObject = prev })
	lookupStorageObject = func(_ context.Context, _ string, _ catalog.StorageObjectFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "s3-standard-use1", Provider: "aws", Service: "s3",
			ResourceName: "standard", Region: "us-east-1",
			Prices: []catalog.Price{
				{Dimension: "storage", Amount: 0.023, Unit: "gb-mo"},
				{Dimension: "requests-put", Amount: 5e-6, Unit: "requests"},
				{Dimension: "requests-get", Amount: 4e-7, Unit: "requests"},
			},
		}}, nil
	}
	it, _ := ParseItem("aws/s3:standard:region=us-east-1:gb_month=500:put_requests=10000:get_requests=100000")
	li, err := storageObjectEstimator{}.Estimate(context.Background(), it)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	want := 0.023*500 + 5e-6*10000 + 4e-7*100000
	if math.Abs(li.MonthlyUSD-want) > 1e-9 {
		t.Fatalf("monthly = %v, want %v", li.MonthlyUSD, want)
	}
}

func TestStorageObjectEstimator_gcpAliases(t *testing.T) {
	prev := lookupStorageObject
	t.Cleanup(func() { lookupStorageObject = prev })
	lookupStorageObject = func(_ context.Context, _ string, _ catalog.StorageObjectFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "gcs-standard-use1", Provider: "gcp", Service: "gcs",
			ResourceName: "standard", Region: "us-east1",
			Prices: []catalog.Price{
				{Dimension: "storage", Amount: 0.02, Unit: "gb-mo"},
				{Dimension: "write-ops", Amount: 4e-6, Unit: "requests"},
				{Dimension: "read-ops", Amount: 5e-7, Unit: "requests"},
			},
		}}, nil
	}
	it, _ := ParseItem("gcp/gcs:standard:region=us-east1:gb_month=100:put_requests=1000:get_requests=2000")
	li, err := storageObjectEstimator{}.Estimate(context.Background(), it)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	want := 0.02*100 + 4e-6*1000 + 5e-7*2000
	if math.Abs(li.MonthlyUSD-want) > 1e-9 {
		t.Fatalf("monthly = %v, want %v", li.MonthlyUSD, want)
	}
}
