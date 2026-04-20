package sku

import (
	"context"
	"testing"

	"github.com/sofq/sku/internal/batch"
)

func TestHandleCompareEstimate_registered(t *testing.T) {
	for _, n := range []string{"compare", "estimate"} {
		if _, ok := batch.Lookup(n); !ok {
			t.Fatalf("%q not registered", n)
		}
	}
}

func TestHandleEstimate_validatesItems(t *testing.T) {
	_, err := handleEstimate(context.Background(), map[string]any{}, batch.Env{Settings: &batch.Settings{}})
	if err == nil {
		t.Fatal("expected error when no items provided")
	}
}
