package estimate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubCosmosLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.NoSQLDBFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupCosmos
	lookupCosmos = fn
	t.Cleanup(func() { lookupCosmos = prev })
}

func TestCosmosEstimator_Provisioned(t *testing.T) {
	resetRegistry(t)
	Register(cosmosEstimator{})
	e, _ := Get("db.nosql.cosmos")
	stubCosmosLookup(t, func(ctx context.Context, shard string, f catalog.NoSQLDBFilter) ([]catalog.Row, error) {
		require.Equal(t, catalog.Terms{Commitment: "on_demand", Tenancy: "sql", OS: "provisioned"}, f.Terms)
		return []catalog.Row{{
			SKUID: "sku-x", Provider: "azure", Service: "cosmosdb",
			ResourceName: "cosmos-provisioned",
			Prices:       []catalog.Price{{Dimension: "provisioned", Amount: 0.008, Unit: "hour"}},
		}}, nil
	})
	item, err := ParseItem("azure/cosmosdb:provisioned:region=eastus:api=sql:ru_per_sec=1000:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	// 1000 RU/s × $0.008/RU/s/hr × 730 hr
	require.InDelta(t, 1000*0.008*730, li.MonthlyUSD, 1e-6)
}

func TestCosmosEstimator_Serverless(t *testing.T) {
	resetRegistry(t)
	Register(cosmosEstimator{})
	e, _ := Get("db.nosql.cosmos")
	stubCosmosLookup(t, func(ctx context.Context, shard string, f catalog.NoSQLDBFilter) ([]catalog.Row, error) {
		require.Equal(t, catalog.Terms{Commitment: "on_demand", Tenancy: "sql", OS: "serverless"}, f.Terms)
		return []catalog.Row{{
			SKUID: "sku-y", Provider: "azure", Service: "cosmosdb",
			ResourceName: "cosmos-serverless",
			Prices:       []catalog.Price{{Dimension: "serverless", Amount: 0.25, Unit: "1m"}},
		}}, nil
	})
	item, err := ParseItem("azure/cosmosdb:serverless:region=eastus:api=sql:ru_million=50")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 50*0.25, li.MonthlyUSD, 1e-6)
}
