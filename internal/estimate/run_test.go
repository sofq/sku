package estimate

import (
	"context"
	"testing"

	"github.com/sofq/sku/internal/catalog"
)

func TestRun_aggregates(t *testing.T) {
	prev := lookupVM
	t.Cleanup(func() { lookupVM = prev })
	lookupVM = func(_ context.Context, _ string, _ catalog.VMFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			Prices: []catalog.Price{{Dimension: "compute", Amount: 0.10, Unit: "hour"}},
		}}, nil
	}
	a, _ := ParseItem("aws/ec2:m5.large:region=us-east-1:hours=100")
	b, _ := ParseItem("aws/ec2:m5.large:region=us-east-1:hours=50:count=2")
	res, err := Run(context.Background(), Config{Items: []Item{a, b}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(res.Items))
	}
	if res.MonthlyTotalUSD != 20.0 {
		t.Fatalf("total = %v, want 20", res.MonthlyTotalUSD)
	}
	if res.Currency != "USD" {
		t.Fatalf("currency = %q", res.Currency)
	}
}

func TestRun_shortCircuitsOnError(t *testing.T) {
	prev := lookupVM
	t.Cleanup(func() { lookupVM = prev })
	lookupVM = func(_ context.Context, _ string, _ catalog.VMFilter) ([]catalog.Row, error) {
		return nil, nil
	}
	a, _ := ParseItem("aws/ec2:m5.large:region=us-east-1")
	_, err := Run(context.Background(), Config{Items: []Item{a}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
