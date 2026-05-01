package estimate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubAPIGatewayLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.APIGatewayFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupAPIGateway
	lookupAPIGateway = fn
	t.Cleanup(func() { lookupAPIGateway = prev })
}

// --- AWS API Gateway: REST per-call with tier crossing ---

func TestAPIGateway_REST_TierCrossing(t *testing.T) {
	resetRegistry(t)
	Register(apiGatewayEstimator{})
	e, ok := Get("api.gateway")
	require.True(t, ok)

	stubAPIGatewayLookup(t, func(_ context.Context, shard string, f catalog.APIGatewayFilter) ([]catalog.Row, error) {
		require.Equal(t, "aws-api-gateway", shard)
		require.Equal(t, "rest", f.ResourceName)
		require.Equal(t, "us-east-1", f.Region)
		return []catalog.Row{{
			SKUID: "apigw-rest-use1", Provider: "aws", Service: "api-gateway",
			ResourceName: "rest", Region: "us-east-1",
			Prices: []catalog.Price{
				{Dimension: "request", Tier: "0", TierUpper: "333M", Amount: 0.0000035, Unit: "request"},
				{Dimension: "request", Tier: "333M", TierUpper: "1B", Amount: 0.00000319, Unit: "request"},
				{Dimension: "request", Tier: "1B", TierUpper: "20B", Amount: 0.00000271, Unit: "request"},
				{Dimension: "request", Tier: "20B", TierUpper: "", Amount: 0.00000172, Unit: "request"},
			},
		}}, nil
	})

	// 500M requests: crosses 333M boundary
	item, err := ParseItem("aws/api-gateway:rest:region=us-east-1:requests=500000000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	expected := 333e6*0.0000035 + 167e6*0.00000319
	require.InDelta(t, expected, li.MonthlyUSD, 1e-3)
	require.Equal(t, "apigw-rest-use1", li.SKUID)
}

// --- AWS API Gateway: HTTP single tier ---

func TestAPIGateway_HTTP_SingleTier(t *testing.T) {
	resetRegistry(t)
	Register(apiGatewayEstimator{})
	e, _ := Get("api.gateway")

	stubAPIGatewayLookup(t, func(_ context.Context, _ string, _ catalog.APIGatewayFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "apigw-http-use1", Provider: "aws", Service: "api-gateway",
			ResourceName: "http", Region: "us-east-1",
			Prices: []catalog.Price{
				{Dimension: "request", Tier: "0", TierUpper: "300M", Amount: 0.000001, Unit: "request"},
				{Dimension: "request", Tier: "300M", TierUpper: "", Amount: 0.0000009, Unit: "request"},
			},
		}}, nil
	})

	item, err := ParseItem("aws/api-gateway:http:region=us-east-1:requests=100000000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 1e8*0.000001, li.MonthlyUSD, 1e-6)
	require.Equal(t, "request", li.QuantityUnit)
}

// --- Azure APIM: provisioned unit-hour ---

func TestAPIGateway_APIM_UnitHour(t *testing.T) {
	resetRegistry(t)
	Register(apiGatewayEstimator{})
	e, _ := Get("api.gateway")

	stubAPIGatewayLookup(t, func(_ context.Context, shard string, _ catalog.APIGatewayFilter) ([]catalog.Row, error) {
		require.Equal(t, "azure-apim", shard)
		return []catalog.Row{{
			SKUID: "apim-dev-eastus", Provider: "azure", Service: "apim",
			ResourceName: "developer", Region: "eastus",
			Prices: []catalog.Price{
				{Dimension: "unit_hour", Tier: "0", TierUpper: "", Amount: 0.07, Unit: "hr"},
			},
		}}, nil
	})

	item, err := ParseItem("azure/apim:developer:region=eastus:units=2:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.07*2*730, li.MonthlyUSD, 1e-6)
	require.Equal(t, "unit-hr", li.QuantityUnit)
	require.InDelta(t, 0.07, li.HourlyUSD, 1e-6)
}

// --- Azure APIM: consumption per-call ---

func TestAPIGateway_APIM_Consumption_PerCall(t *testing.T) {
	resetRegistry(t)
	Register(apiGatewayEstimator{})
	e, _ := Get("api.gateway")

	stubAPIGatewayLookup(t, func(_ context.Context, _ string, _ catalog.APIGatewayFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "apim-cons-eastus", Provider: "azure", Service: "apim",
			ResourceName: "consumption", Region: "eastus",
			Prices: []catalog.Price{
				{Dimension: "call", Tier: "0", TierUpper: "", Amount: 0.0000035, Unit: "call"},
			},
		}}, nil
	})

	item, err := ParseItem("azure/apim:consumption:region=eastus:requests=1000000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 1e6*0.0000035, li.MonthlyUSD, 1e-6)
	require.Equal(t, "call", li.QuantityUnit)
}

// --- Cross-mode input validation ---

func TestAPIGateway_APIM_RequestsOnProvisioned_Error(t *testing.T) {
	resetRegistry(t)
	Register(apiGatewayEstimator{})
	e, _ := Get("api.gateway")

	stubAPIGatewayLookup(t, func(_ context.Context, _ string, _ catalog.APIGatewayFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "apim-dev-eastus", Provider: "azure", Service: "apim",
			ResourceName: "developer", Region: "eastus",
			Prices: []catalog.Price{
				{Dimension: "unit_hour", Tier: "0", TierUpper: "", Amount: 0.07, Unit: "hr"},
			},
		}}, nil
	})

	item, err := ParseItem("azure/apim:developer:region=eastus:requests=1000")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unit-hour")
}

func TestAPIGateway_AWS_UnitsOnPerCall_Error(t *testing.T) {
	resetRegistry(t)
	Register(apiGatewayEstimator{})
	e, _ := Get("api.gateway")

	stubAPIGatewayLookup(t, func(_ context.Context, _ string, _ catalog.APIGatewayFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "apigw-rest-use1", Provider: "aws", Service: "api-gateway",
			ResourceName: "rest", Region: "us-east-1",
			Prices: []catalog.Price{
				{Dimension: "request", Tier: "0", TierUpper: "", Amount: 0.0000035},
			},
		}}, nil
	})

	// Provide requests AND units/hours to trigger the per-call cross-validation.
	item, err := ParseItem("aws/api-gateway:rest:region=us-east-1:requests=1000:units=2:hours=730")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "per-call")
}

// --- Missing region ---

func TestAPIGateway_MissingRegion(t *testing.T) {
	resetRegistry(t)
	Register(apiGatewayEstimator{})
	e, _ := Get("api.gateway")

	item, err := ParseItem("aws/api-gateway:rest:requests=1000")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "region")
}

// --- Unknown shard ---

func TestAPIGateway_UnknownShard(t *testing.T) {
	resetRegistry(t)
	Register(apiGatewayEstimator{})
	e, _ := Get("api.gateway")

	// Build Item directly — provider/service not in shard map.
	item := Item{
		Raw:      "gcp/api-gateway:rest:region=us-east1:requests=1000",
		Provider: "gcp", Service: "api-gateway",
		Resource: "rest", Kind: "api.gateway",
		Params: map[string]string{"region": "us-east1", "requests": "1000"},
	}
	_, err := e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no shard")
}

// --- No requests param ---

func TestAPIGateway_NoRequests(t *testing.T) {
	resetRegistry(t)
	Register(apiGatewayEstimator{})
	e, _ := Get("api.gateway")

	stubAPIGatewayLookup(t, func(_ context.Context, _ string, _ catalog.APIGatewayFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "apigw-rest-use1", Provider: "aws", Service: "api-gateway",
			ResourceName: "rest", Region: "us-east-1",
			Prices: []catalog.Price{
				{Dimension: "request", Tier: "0", TierUpper: "", Amount: 0.0000035},
			},
		}}, nil
	})

	item, err := ParseItem("aws/api-gateway:rest:region=us-east-1")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requests")
}
