package estimate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubOpenSearchLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.SearchEngineFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupOpenSearch
	lookupOpenSearch = fn
	t.Cleanup(func() { lookupOpenSearch = prev })
}

func TestOpenSearchEstimator_ManagedCluster(t *testing.T) {
	resetRegistry(t)
	Register(opensearchEstimator{})
	e, ok := Get("search.engine.opensearch")
	require.True(t, ok)

	stubOpenSearchLookup(t, func(_ context.Context, shard string, f catalog.SearchEngineFilter) ([]catalog.Row, error) {
		require.Equal(t, "aws-opensearch", shard)
		require.Equal(t, "r6g.large.search", f.ResourceName)
		require.Equal(t, "managed-cluster", f.Terms.OS)
		return []catalog.Row{{
			SKUID: "sku-os-1", Provider: "aws", Service: "opensearch",
			ResourceName: "r6g.large.search", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "instance", Amount: 0.166, Unit: "hour"}},
		}}, nil
	})

	item, err := ParseItem("aws/opensearch:r6g.large.search:region=us-east-1:count=3:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.166*730*3, li.MonthlyUSD, 1e-6)
	require.Equal(t, "sku-os-1", li.SKUID)
}

func TestOpenSearchEstimator_ManagedClusterDefaultCount(t *testing.T) {
	resetRegistry(t)
	Register(opensearchEstimator{})
	e, _ := Get("search.engine.opensearch")

	stubOpenSearchLookup(t, func(_ context.Context, _ string, _ catalog.SearchEngineFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "sku-os-2", Provider: "aws", Service: "opensearch",
			ResourceName: "m5.large.search", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "instance", Amount: 0.134, Unit: "hour"}},
		}}, nil
	})

	item, err := ParseItem("aws/opensearch:m5.large.search:region=us-east-1:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.134*730, li.MonthlyUSD, 1e-6)
}

func TestOpenSearchEstimator_Serverless(t *testing.T) {
	resetRegistry(t)
	Register(opensearchEstimator{})
	e, _ := Get("search.engine.opensearch")

	stubOpenSearchLookup(t, func(_ context.Context, _ string, f catalog.SearchEngineFilter) ([]catalog.Row, error) {
		require.Equal(t, "opensearch-serverless", f.ResourceName)
		require.Equal(t, "serverless", f.Terms.OS)
		return []catalog.Row{{
			SKUID: "sku-os-sl", Provider: "aws", Service: "opensearch",
			ResourceName: "opensearch-serverless", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "ocu", Amount: 0.24, Unit: "hour"}},
		}}, nil
	})

	item, err := ParseItem("aws/opensearch:serverless:region=us-east-1:ocu_hours=720")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.24*720, li.MonthlyUSD, 1e-6)
	require.Equal(t, "ocu-hour", li.QuantityUnit)
}

func TestOpenSearchEstimator_ServerlessMissingOCUHours(t *testing.T) {
	resetRegistry(t)
	Register(opensearchEstimator{})
	e, _ := Get("search.engine.opensearch")

	stubOpenSearchLookup(t, func(_ context.Context, _ string, _ catalog.SearchEngineFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "sku-os-sl", Provider: "aws", Service: "opensearch",
			ResourceName: "opensearch-serverless", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "ocu", Amount: 0.24, Unit: "hour"}},
		}}, nil
	})

	item, err := ParseItem("aws/opensearch:serverless:region=us-east-1")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ocu_hours")
}

func TestOpenSearchEstimator_MissingRegion(t *testing.T) {
	resetRegistry(t)
	Register(opensearchEstimator{})
	e, _ := Get("search.engine.opensearch")

	item, err := ParseItem("aws/opensearch:r6g.large.search:hours=730")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "region")
}

func TestOpenSearchEstimator_NoRows(t *testing.T) {
	resetRegistry(t)
	Register(opensearchEstimator{})
	e, _ := Get("search.engine.opensearch")

	stubOpenSearchLookup(t, func(_ context.Context, _ string, _ catalog.SearchEngineFilter) ([]catalog.Row, error) {
		return nil, nil
	})

	item, err := ParseItem("aws/opensearch:r6g.large.search:region=us-east-1")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
}
