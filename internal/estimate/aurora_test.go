package estimate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubAuroraLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.DBRelationalFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupAurora
	lookupAurora = fn
	t.Cleanup(func() { lookupAurora = prev })
}

func TestAuroraEstimator_Provisioned(t *testing.T) {
	resetRegistry(t)
	Register(auroraEstimator{})
	e, ok := Get("db.relational.aurora")
	require.True(t, ok)

	stubAuroraLookup(t, func(_ context.Context, shard string, f catalog.DBRelationalFilter) ([]catalog.Row, error) {
		require.Equal(t, "aws-aurora", shard)
		require.Equal(t, "db.r6g.large", f.InstanceType)
		require.Equal(t, catalog.Terms{Commitment: "on_demand", Tenancy: "aurora-postgres", OS: "single-az"}, f.Terms)
		return []catalog.Row{{
			SKUID: "sku-1", Provider: "aws", Service: "aurora",
			ResourceName: "db.r6g.large", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "compute", Amount: 0.50, Unit: "hour"}},
		}}, nil
	})
	item, err := ParseItem("aws/aurora:db.r6g.large:region=us-east-1:hours=730:count=2:engine=aurora-postgres")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.50*730*2, li.MonthlyUSD, 1e-6)
}

func TestAuroraEstimator_ServerlessV2(t *testing.T) {
	resetRegistry(t)
	Register(auroraEstimator{})
	e, _ := Get("db.relational.aurora")

	stubAuroraLookup(t, func(_ context.Context, _ string, f catalog.DBRelationalFilter) ([]catalog.Row, error) {
		require.Equal(t, "aurora-serverless-v2", f.InstanceType)
		return []catalog.Row{{
			SKUID: "sku-2", Provider: "aws", Service: "aurora",
			ResourceName: "aurora-serverless-v2", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "compute", Amount: 0.12, Unit: "hour"}},
		}}, nil
	})
	item, err := ParseItem("aws/aurora:serverless-v2:region=us-east-1:acu_hours=8000:engine=aurora-postgres")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.12*8000, li.MonthlyUSD, 1e-6)
}
