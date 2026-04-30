package estimate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubAppServiceLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.PaasAppFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupAppService
	lookupAppService = fn
	t.Cleanup(func() { lookupAppService = prev })
}

func TestAppServiceEstimator_Linux(t *testing.T) {
	resetRegistry(t)
	Register(appServiceEstimator{})
	e, ok := Get("paas.app.appservice")
	require.True(t, ok)

	stubAppServiceLookup(t, func(_ context.Context, shard string, f catalog.PaasAppFilter) ([]catalog.Row, error) {
		require.Equal(t, "azure-appservice", shard)
		require.Equal(t, "P1v3", f.ResourceName)
		require.Equal(t, "linux", f.Terms.OS)
		return []catalog.Row{{
			SKUID: "sku-as-1", Provider: "azure", Service: "appservice",
			ResourceName: "P1v3", Region: "eastus",
			Prices: []catalog.Price{{Dimension: "instance", Amount: 0.169, Unit: "hour"}},
		}}, nil
	})

	item, err := ParseItem("azure/appservice:P1v3:region=eastus:os=linux:count=2:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.169*730*2, li.MonthlyUSD, 1e-6)
	require.Equal(t, "sku-as-1", li.SKUID)
}

func TestAppServiceEstimator_WindowsDefaultsOS(t *testing.T) {
	resetRegistry(t)
	Register(appServiceEstimator{})
	e, _ := Get("paas.app.appservice")

	stubAppServiceLookup(t, func(_ context.Context, _ string, f catalog.PaasAppFilter) ([]catalog.Row, error) {
		// Default OS is linux when not specified.
		require.Equal(t, "linux", f.Terms.OS)
		return []catalog.Row{{
			SKUID: "sku-as-2", Provider: "azure", Service: "appservice",
			ResourceName: "P2v3", Region: "eastus",
			Prices: []catalog.Price{{Dimension: "instance", Amount: 0.338, Unit: "hour"}},
		}}, nil
	})

	item, err := ParseItem("azure/appservice:P2v3:region=eastus:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.338*730, li.MonthlyUSD, 1e-6)
}

func TestAppServiceEstimator_TierFilter(t *testing.T) {
	resetRegistry(t)
	Register(appServiceEstimator{})
	e, _ := Get("paas.app.appservice")

	stubAppServiceLookup(t, func(_ context.Context, _ string, f catalog.PaasAppFilter) ([]catalog.Row, error) {
		require.Equal(t, "premiumv3", f.Terms.SupportTier)
		return []catalog.Row{{
			SKUID: "sku-as-3", Provider: "azure", Service: "appservice",
			ResourceName: "P1v3", Region: "westus",
			Prices: []catalog.Price{{Dimension: "instance", Amount: 0.169, Unit: "hour"}},
		}}, nil
	})

	item, err := ParseItem("azure/appservice:P1v3:region=westus:tier=premiumv3:hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.169*730, li.MonthlyUSD, 1e-6)
}

func TestAppServiceEstimator_MissingRegion(t *testing.T) {
	resetRegistry(t)
	Register(appServiceEstimator{})
	e, _ := Get("paas.app.appservice")

	item, err := ParseItem("azure/appservice:P1v3:hours=730")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "region")
}

func TestAppServiceEstimator_NoRows(t *testing.T) {
	resetRegistry(t)
	Register(appServiceEstimator{})
	e, _ := Get("paas.app.appservice")

	stubAppServiceLookup(t, func(_ context.Context, _ string, _ catalog.PaasAppFilter) ([]catalog.Row, error) {
		return nil, nil
	})

	item, err := ParseItem("azure/appservice:P1v3:region=eastus")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
}
