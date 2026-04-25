package estimate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubSpannerLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.DBRelationalFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupSpanner
	lookupSpanner = fn
	t.Cleanup(func() { lookupSpanner = prev })
}

func TestSpannerEstimator_PerPU(t *testing.T) {
	resetRegistry(t)
	Register(spannerEstimator{})
	e, _ := Get("db.relational.spanner")
	stubSpannerLookup(t, func(_ context.Context, _ string, f catalog.DBRelationalFilter) ([]catalog.Row, error) {
		require.Equal(t, "spanner-standard", f.InstanceType)
		require.Equal(t, catalog.Terms{Commitment: "on_demand", Tenancy: "spanner-standard"}, f.Terms)
		return []catalog.Row{{
			SKUID: "sku-z", Provider: "gcp", Service: "spanner",
			ResourceName: "spanner-standard",
			Prices:       []catalog.Price{{Dimension: "compute", Amount: 0.0009, Unit: "pu-hour"}},
		}}, nil
	})
	item, err := ParseItem("gcp/spanner:standard:region=us-east1:pu=1000:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 1000*0.0009*730, li.MonthlyUSD, 1e-6)
}

func TestSpannerEstimator_PerNodeSugar(t *testing.T) {
	resetRegistry(t)
	Register(spannerEstimator{})
	e, _ := Get("db.relational.spanner")
	stubSpannerLookup(t, func(_ context.Context, _ string, f catalog.DBRelationalFilter) ([]catalog.Row, error) {
		require.Equal(t, catalog.Terms{Commitment: "on_demand", Tenancy: "spanner-enterprise"}, f.Terms)
		return []catalog.Row{{
			SKUID: "sku-z2", Provider: "gcp", Service: "spanner",
			ResourceName: "spanner-enterprise",
			Prices:       []catalog.Price{{Dimension: "compute", Amount: 0.00135, Unit: "pu-hour"}},
		}}, nil
	})
	item, err := ParseItem("gcp/spanner:enterprise:region=us-east1:node=2:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	// node=2 → 2000 PU × 0.00135 × 730
	require.InDelta(t, 2000*0.00135*730, li.MonthlyUSD, 1e-6)
}
