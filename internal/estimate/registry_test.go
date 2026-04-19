package estimate

import (
	"context"
	"testing"
)

type fakeEstimator struct{ kind string }

func (f fakeEstimator) Kind() string { return f.kind }
func (f fakeEstimator) Estimate(_ context.Context, it Item) (LineItem, error) {
	return LineItem{Item: it, Kind: f.kind, HourlyUSD: 1, Quantity: 1, MonthlyUSD: 1}, nil
}

func TestRegistry_registerAndGet(t *testing.T) {
	resetRegistry(t)
	Register(fakeEstimator{kind: "compute.vm"})
	e, ok := Get("compute.vm")
	if !ok {
		t.Fatal("estimator not registered")
	}
	if e.Kind() != "compute.vm" {
		t.Fatalf("wrong kind: %q", e.Kind())
	}
}

func TestRegistry_missing(t *testing.T) {
	resetRegistry(t)
	if _, ok := Get("storage.object"); ok {
		t.Fatal("expected miss for unregistered kind")
	}
}
