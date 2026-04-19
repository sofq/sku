package estimate

import (
	"context"
	"math"
	"testing"

	"github.com/sofq/sku/internal/catalog"
)

func TestVMEstimator_monthlyDefault(t *testing.T) {
	resetRegistry(t)
	prev := lookupVM
	t.Cleanup(func() { lookupVM = prev })
	lookupVM = func(_ context.Context, _ string, _ catalog.VMFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "sku-1", Provider: "aws", Service: "ec2",
			ResourceName: "m5.large", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "compute", Amount: 0.096, Unit: "hour"}},
		}}, nil
	}

	e := vmEstimator{}
	it, err := ParseItem("aws/ec2:m5.large:region=us-east-1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	li, err := e.Estimate(context.Background(), it)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	want := 0.096 * 1 * 730
	if math.Abs(li.MonthlyUSD-want) > 1e-9 {
		t.Fatalf("monthly = %v, want %v", li.MonthlyUSD, want)
	}
	if li.HourlyUSD != 0.096 || li.Quantity != 730 || li.QuantityUnit != "hour" {
		t.Fatalf("bad line: %+v", li)
	}
}

func TestVMEstimator_countAndHours(t *testing.T) {
	prev := lookupVM
	t.Cleanup(func() { lookupVM = prev })
	lookupVM = func(_ context.Context, _ string, _ catalog.VMFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			Prices: []catalog.Price{{Dimension: "compute", Amount: 0.10, Unit: "hour"}},
		}}, nil
	}
	it, _ := ParseItem("aws/ec2:m5.large:region=us-east-1:count=3:hours=100")
	li, err := vmEstimator{}.Estimate(context.Background(), it)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if math.Abs(li.MonthlyUSD-30.0) > 1e-9 {
		t.Fatalf("monthly = %v, want 30.0", li.MonthlyUSD)
	}
}

func TestVMEstimator_regionRequired(t *testing.T) {
	it, _ := ParseItem("aws/ec2:m5.large")
	if _, err := (vmEstimator{}).Estimate(context.Background(), it); err == nil {
		t.Fatal("expected region-required error")
	}
}

func TestVMEstimator_ambiguousRows(t *testing.T) {
	prev := lookupVM
	t.Cleanup(func() { lookupVM = prev })
	lookupVM = func(_ context.Context, _ string, _ catalog.VMFilter) ([]catalog.Row, error) {
		return []catalog.Row{{SKUID: "a"}, {SKUID: "b"}}, nil
	}
	it, _ := ParseItem("aws/ec2:m5.large:region=us-east-1")
	if _, err := (vmEstimator{}).Estimate(context.Background(), it); err == nil {
		t.Fatal("expected ambiguous error")
	}
}
