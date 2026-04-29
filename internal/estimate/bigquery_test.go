package estimate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubBigQueryLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.WarehouseQueryFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupBigQuery
	lookupBigQuery = fn
	t.Cleanup(func() { lookupBigQuery = prev })
}

func TestBigQueryEstimator_OnDemand(t *testing.T) {
	resetRegistry(t)
	Register(bigqueryEstimator{})
	e, ok := Get("warehouse.query.bigquery")
	require.True(t, ok)

	stubBigQueryLookup(t, func(_ context.Context, shard string, f catalog.WarehouseQueryFilter) ([]catalog.Row, error) {
		require.Equal(t, "gcp-bigquery", shard)
		require.Equal(t, "on-demand", f.ResourceName)
		require.Equal(t, "on-demand", f.Terms.OS)
		return []catalog.Row{{
			SKUID: "sku-bq-od", Provider: "gcp", Service: "bigquery",
			ResourceName: "on-demand", Region: "bq-us",
			Prices: []catalog.Price{{Dimension: "query", Amount: 5.0, Unit: "tb"}},
		}}, nil
	})

	item, err := ParseItem("gcp/bigquery:on-demand:region=bq-us:tb_queried=10")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 5.0*10, li.MonthlyUSD, 1e-6)
	require.Equal(t, "tb", li.QuantityUnit)
}

func TestBigQueryEstimator_OnDemandMissingTB(t *testing.T) {
	resetRegistry(t)
	Register(bigqueryEstimator{})
	e, _ := Get("warehouse.query.bigquery")

	stubBigQueryLookup(t, func(_ context.Context, _ string, _ catalog.WarehouseQueryFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "sku-bq-od", Provider: "gcp", Service: "bigquery",
			ResourceName: "on-demand", Region: "bq-us",
			Prices: []catalog.Price{{Dimension: "query", Amount: 5.0, Unit: "tb"}},
		}}, nil
	})

	item, err := ParseItem("gcp/bigquery:on-demand:region=bq-us")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tb_queried")
}

func TestBigQueryEstimator_CapacityStandard(t *testing.T) {
	resetRegistry(t)
	Register(bigqueryEstimator{})
	e, _ := Get("warehouse.query.bigquery")

	stubBigQueryLookup(t, func(_ context.Context, _ string, f catalog.WarehouseQueryFilter) ([]catalog.Row, error) {
		require.Equal(t, "capacity-standard", f.ResourceName)
		return []catalog.Row{{
			SKUID: "sku-bq-cap-std", Provider: "gcp", Service: "bigquery",
			ResourceName: "capacity-standard", Region: "bq-us",
			Prices: []catalog.Price{{Dimension: "slot", Amount: 0.034, Unit: "slot-hour"}},
		}}, nil
	})

	item, err := ParseItem("gcp/bigquery:capacity-standard:region=bq-us:slots=100:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.034*100*730, li.MonthlyUSD, 1e-6)
	require.Equal(t, "slot-hour", li.QuantityUnit)
}

func TestBigQueryEstimator_CapacityEnterprise(t *testing.T) {
	resetRegistry(t)
	Register(bigqueryEstimator{})
	e, _ := Get("warehouse.query.bigquery")

	stubBigQueryLookup(t, func(_ context.Context, _ string, _ catalog.WarehouseQueryFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "sku-bq-cap-ent", Provider: "gcp", Service: "bigquery",
			ResourceName: "capacity-enterprise", Region: "bq-us",
			Prices: []catalog.Price{{Dimension: "slot", Amount: 0.060, Unit: "slot-hour"}},
		}}, nil
	})

	item, err := ParseItem("gcp/bigquery:capacity-enterprise:region=bq-us:slots=500:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.060*500*730, li.MonthlyUSD, 1e-6)
}

func TestBigQueryEstimator_StorageActive(t *testing.T) {
	resetRegistry(t)
	Register(bigqueryEstimator{})
	e, _ := Get("warehouse.query.bigquery")

	stubBigQueryLookup(t, func(_ context.Context, _ string, f catalog.WarehouseQueryFilter) ([]catalog.Row, error) {
		require.Equal(t, "storage-active", f.ResourceName)
		return []catalog.Row{{
			SKUID: "sku-bq-stor-act", Provider: "gcp", Service: "bigquery",
			ResourceName: "storage-active", Region: "bq-us",
			Prices: []catalog.Price{{Dimension: "storage", Amount: 0.020, Unit: "gb-month"}},
		}}, nil
	})

	item, err := ParseItem("gcp/bigquery:storage-active:region=bq-us:gb_month=5000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.020*5000, li.MonthlyUSD, 1e-6)
	require.Equal(t, "gb-month", li.QuantityUnit)
}

func TestBigQueryEstimator_StorageLongTerm(t *testing.T) {
	resetRegistry(t)
	Register(bigqueryEstimator{})
	e, _ := Get("warehouse.query.bigquery")

	stubBigQueryLookup(t, func(_ context.Context, _ string, _ catalog.WarehouseQueryFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "sku-bq-stor-lt", Provider: "gcp", Service: "bigquery",
			ResourceName: "storage-long-term", Region: "bq-eu",
			Prices: []catalog.Price{{Dimension: "storage", Amount: 0.010, Unit: "gb-month"}},
		}}, nil
	})

	item, err := ParseItem("gcp/bigquery:storage-long-term:region=bq-eu:gb_month=10000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.010*10000, li.MonthlyUSD, 1e-6)
}

func TestBigQueryEstimator_InvalidResource(t *testing.T) {
	resetRegistry(t)
	Register(bigqueryEstimator{})
	e, _ := Get("warehouse.query.bigquery")

	item, err := ParseItem("gcp/bigquery:unknown-mode:region=bq-us:tb_queried=10")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource must be")
}

func TestBigQueryEstimator_MissingRegion(t *testing.T) {
	resetRegistry(t)
	Register(bigqueryEstimator{})
	e, _ := Get("warehouse.query.bigquery")

	item, err := ParseItem("gcp/bigquery:on-demand:tb_queried=10")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "region")
}
