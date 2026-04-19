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
