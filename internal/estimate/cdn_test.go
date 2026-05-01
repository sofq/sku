package estimate

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubCDNLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.CDNFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupCDN
	lookupCDN = fn
	t.Cleanup(func() { lookupCDN = prev })
}

// rowWithMode creates a catalog.Row with extra.mode set.
func rowWithMode(skuID, provider, service, resource, region, mode string, prices []catalog.Price) catalog.Row {
	return catalog.Row{
		SKUID: skuID, Provider: provider, Service: service,
		ResourceName: resource, Region: region,
		ResourceAttrs: catalog.ResourceAttrs{Extra: map[string]any{"mode": mode}},
		Prices:        prices,
	}
}

// rowWithModeSku creates a catalog.Row with extra.mode and extra.sku set.
func rowWithModeSku(skuID, provider, service, resource, region, mode, skuToken string, prices []catalog.Price) catalog.Row {
	return catalog.Row{
		SKUID: skuID, Provider: provider, Service: service,
		ResourceName: resource, Region: region,
		ResourceAttrs: catalog.ResourceAttrs{Extra: map[string]any{"mode": mode, "sku": skuToken}},
		Prices:        prices,
	}
}

// --- AWS CloudFront: tier-0 happy path ---

func TestCDN_CloudFront_Tier0_HappyPath(t *testing.T) {
	resetRegistry(t)
	Register(networkCDNTier0Estimator{})
	e, ok := Get("network.cdn")
	require.True(t, ok)

	stubCDNLookup(t, func(_ context.Context, shard string, f catalog.CDNFilter) ([]catalog.Row, error) {
		require.Equal(t, "aws-cloudfront", shard)
		require.Equal(t, "us-east-1", f.Region)
		// CloudFront: no base-fee, so global lookup returns empty
		if f.Region == "global" {
			return nil, nil
		}
		return []catalog.Row{
			rowWithMode("cf-use1", "aws", "cloudfront", "cloudfront", "us-east-1", "edge-egress", []catalog.Price{
				{Dimension: "egress", Tier: "0", TierUpper: "10TB", Amount: 0.085, Unit: "gb"},
				{Dimension: "egress", Tier: "10TB", TierUpper: "", Amount: 0.08, Unit: "gb"},
			}),
		}, nil
	})

	item, err := ParseItem("aws/cloudfront:cloudfront:region=us-east-1:gb=100")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 100*0.085, li.MonthlyUSD, 1e-6)
	require.Equal(t, "cf-use1", li.SKUID)
	require.Equal(t, "gb", li.QuantityUnit)
}

// --- AWS CloudFront: base-fee join returns $0 (no base-fee rows) ---

func TestCDN_CloudFront_BaseFeeSumZero(t *testing.T) {
	resetRegistry(t)
	Register(networkCDNTier0Estimator{})
	e, _ := Get("network.cdn")

	stubCDNLookup(t, func(_ context.Context, _ string, f catalog.CDNFilter) ([]catalog.Row, error) {
		if f.Region == "global" {
			return nil, nil // no base-fee rows
		}
		return []catalog.Row{
			rowWithMode("cf-use1", "aws", "cloudfront", "cloudfront", "us-east-1", "edge-egress", []catalog.Price{
				{Dimension: "egress", Tier: "0", TierUpper: "", Amount: 0.085, Unit: "gb"},
			}),
		}, nil
	})

	item, err := ParseItem("aws/cloudfront:cloudfront:region=us-east-1:gb=50")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 50*0.085, li.MonthlyUSD, 1e-6)
}

// --- Azure Front Door: base-fee join returns expected value ---

func TestCDN_FrontDoor_BaseFeeJoin(t *testing.T) {
	resetRegistry(t)
	Register(networkCDNTier0Estimator{})
	e, _ := Get("network.cdn")

	const skuToken = "front-door-standard"
	stubCDNLookup(t, func(_ context.Context, shard string, f catalog.CDNFilter) ([]catalog.Row, error) {
		require.Equal(t, "azure-front-door", shard)
		if f.Region == "global" {
			// Base-fee row
			return []catalog.Row{
				rowWithModeSku("fd-base", "azure", "front-door", "standard", "global", "base-fee", skuToken, []catalog.Price{
					// Dimension matches azure_front_door.py output ("fee"); the
					// tier-0 estimator sums all amounts so the literal value
					// here doesn't affect the result, but the string must match
					// what the ingestor emits to keep this stub honest.
					{Dimension: "fee", Tier: "0", TierUpper: "", Amount: 35.0, Unit: "month"},
				}),
			}, nil
		}
		// Egress row for region lookup
		return []catalog.Row{
			rowWithModeSku("fd-egress-eus", "azure", "front-door", "standard", "eastus", "edge-egress", skuToken, []catalog.Price{
				{Dimension: "egress", Tier: "0", TierUpper: "10TB", Amount: 0.087, Unit: "gb"},
			}),
		}, nil
	})

	item, err := ParseItem("azure/front-door:standard:region=eastus:gb=100")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	expected := 100*0.087 + 35.0
	require.InDelta(t, expected, li.MonthlyUSD, 1e-6)
}

// --- GCP Cloud CDN: tier-0 happy path ---

func TestCDN_CloudCDN_Tier0_HappyPath(t *testing.T) {
	resetRegistry(t)
	Register(networkCDNTier0Estimator{})
	e, _ := Get("network.cdn")

	stubCDNLookup(t, func(_ context.Context, shard string, f catalog.CDNFilter) ([]catalog.Row, error) {
		require.Equal(t, "gcp-cloud-cdn", shard)
		if f.Region == "global" {
			return nil, nil
		}
		return []catalog.Row{
			rowWithMode("cloud-cdn-na", "gcp", "cloud-cdn", "standard", "us-east1", "edge-egress", []catalog.Price{
				{Dimension: "egress", Tier: "0", TierUpper: "10TB", Amount: 0.08, Unit: "gb"},
				{Dimension: "egress", Tier: "10TB", TierUpper: "", Amount: 0.06, Unit: "gb"},
			}),
		}, nil
	})

	item, err := ParseItem("gcp/cloud-cdn:standard:region=us-east1:gb=500")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 500*0.08, li.MonthlyUSD, 1e-6)
}

// --- Tier-0 boundary exceeded ---

func TestCDN_Tier0_BoundaryExceeded(t *testing.T) {
	resetRegistry(t)
	Register(networkCDNTier0Estimator{})
	e, _ := Get("network.cdn")

	stubCDNLookup(t, func(_ context.Context, _ string, f catalog.CDNFilter) ([]catalog.Row, error) {
		if f.Region == "global" {
			return nil, nil
		}
		return []catalog.Row{
			rowWithMode("cf-use1", "aws", "cloudfront", "cloudfront", "us-east-1", "edge-egress", []catalog.Price{
				{Dimension: "egress", Tier: "0", TierUpper: "10TB", Amount: 0.085, Unit: "gb"},
				{Dimension: "egress", Tier: "10TB", TierUpper: "", Amount: 0.08, Unit: "gb"},
			}),
		}, nil
	})

	// 10TB boundary = 10*1e12/1e9 = 10000 GB
	item, err := ParseItem("aws/cloudfront:cloudfront:region=us-east-1:gb=15000")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tier-0 boundary")
	require.Contains(t, err.Error(), "M-ε")
}

// --- Tier-0 upper bound infinite (no cap) ---

func TestCDN_Tier0_Infinite(t *testing.T) {
	resetRegistry(t)
	Register(networkCDNTier0Estimator{})
	e, _ := Get("network.cdn")

	stubCDNLookup(t, func(_ context.Context, _ string, f catalog.CDNFilter) ([]catalog.Row, error) {
		if f.Region == "global" {
			return nil, nil
		}
		// tier_upper="" → MaxFloat64, so no boundary exceeded
		return []catalog.Row{
			rowWithMode("cf-use1", "aws", "cloudfront", "cloudfront", "us-east-1", "edge-egress", []catalog.Price{
				{Dimension: "egress", Tier: "0", TierUpper: "", Amount: 0.085, Unit: "gb"},
			}),
		}, nil
	})

	item, err := ParseItem("aws/cloudfront:cloudfront:region=us-east-1:gb=99999")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 99999*0.085, li.MonthlyUSD, 1e-2)
}

// --- parseTierBoundBytes unit tests ---

func TestParseTierBoundBytes_Empty(t *testing.T) {
	v, err := parseTierBoundBytes("")
	require.NoError(t, err)
	require.Equal(t, math.MaxFloat64, v)
}

func TestParseTierBoundBytes_KnownToken(t *testing.T) {
	v, err := parseTierBoundBytes("10TB")
	require.NoError(t, err)
	require.InDelta(t, 10*1e12, v, 1e3)
}

func TestParseTierBoundBytes_NumericFallback(t *testing.T) {
	v, err := parseTierBoundBytes("1234567890")
	require.NoError(t, err)
	require.InDelta(t, 1234567890, v, 1)
}

func TestParseTierBoundBytes_InvalidToken(t *testing.T) {
	_, err := parseTierBoundBytes("notanumber")
	require.Error(t, err)
}

// --- Missing params ---

func TestCDN_MissingRegion(t *testing.T) {
	resetRegistry(t)
	Register(networkCDNTier0Estimator{})
	e, _ := Get("network.cdn")

	item, err := ParseItem("aws/cloudfront:cloudfront:gb=100")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "region")
}

func TestCDN_MissingGB(t *testing.T) {
	resetRegistry(t)
	Register(networkCDNTier0Estimator{})
	e, _ := Get("network.cdn")

	item, err := ParseItem("aws/cloudfront:cloudfront:region=us-east-1")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "gb")
}

func TestCDN_UnknownShard(t *testing.T) {
	resetRegistry(t)
	Register(networkCDNTier0Estimator{})
	e, _ := Get("network.cdn")

	// Build Item directly — provider/service not in shard map.
	item := Item{
		Raw:      "azure/cloud-cdn:standard:region=eastus:gb=100",
		Provider: "azure", Service: "cloud-cdn",
		Resource: "standard", Kind: "network.cdn",
		Params: map[string]string{"region": "eastus", "gb": "100"},
	}
	_, err := e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no shard")
}
