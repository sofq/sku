package estimate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubDNSZoneLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.DNSZoneFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupDNSZone
	lookupDNSZone = fn
	t.Cleanup(func() { lookupDNSZone = prev })
}

// --- AWS Route53 ---

func TestDNSZone_Route53_ZonesOnly(t *testing.T) {
	resetRegistry(t)
	Register(dnsZoneEstimator{})
	e, ok := Get("dns.zone")
	require.True(t, ok)

	stubDNSZoneLookup(t, func(_ context.Context, shard string, f catalog.DNSZoneFilter) ([]catalog.Row, error) {
		require.Equal(t, "aws-route53", shard)
		require.Equal(t, "public", f.ResourceName)
		require.Equal(t, "global", f.Region)
		return []catalog.Row{{
			SKUID: "r53-public-global", Provider: "aws", Service: "route53",
			ResourceName: "public", Region: "global",
			Prices: []catalog.Price{
				{Dimension: "hosted_zone", Tier: "0", TierUpper: "", Amount: 0.50, Unit: "mo"},
				{Dimension: "query", Tier: "0", TierUpper: "", Amount: 0.0000004, Unit: "query"},
			},
		}}, nil
	})

	item, err := ParseItem("aws/route53:public:region=global:zones=5")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.50*5, li.MonthlyUSD, 1e-6)
	require.Equal(t, "r53-public-global", li.SKUID)
}

func TestDNSZone_Route53_ZonesAndQueries(t *testing.T) {
	resetRegistry(t)
	Register(dnsZoneEstimator{})
	e, _ := Get("dns.zone")

	stubDNSZoneLookup(t, func(_ context.Context, _ string, _ catalog.DNSZoneFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "r53-public-global", Provider: "aws", Service: "route53",
			ResourceName: "public", Region: "global",
			Prices: []catalog.Price{
				{Dimension: "hosted_zone", Tier: "0", TierUpper: "", Amount: 0.50, Unit: "mo"},
				{Dimension: "query", Tier: "0", TierUpper: "", Amount: 0.0000004, Unit: "query"},
			},
		}}, nil
	})

	item, err := ParseItem("aws/route53:public:region=global:zones=10:queries=1000000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.50*10+0.0000004*1e6, li.MonthlyUSD, 1e-6)
}

func TestDNSZone_Route53_QueriesTierCrossing(t *testing.T) {
	resetRegistry(t)
	Register(dnsZoneEstimator{})
	e, _ := Get("dns.zone")

	stubDNSZoneLookup(t, func(_ context.Context, _ string, _ catalog.DNSZoneFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "r53-public-global", Provider: "aws", Service: "route53",
			ResourceName: "public", Region: "global",
			Prices: []catalog.Price{
				{Dimension: "query", Tier: "0", TierUpper: "1B", Amount: 0.0000004, Unit: "query"},
				{Dimension: "query", Tier: "1B", TierUpper: "", Amount: 0.0000002, Unit: "query"},
			},
		}}, nil
	})

	// 2B queries: first 1B at 0.0000004, next 1B at 0.0000002
	item, err := ParseItem("aws/route53:public:region=global:queries=2000000000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	expected := 1e9*0.0000004 + 1e9*0.0000002
	require.InDelta(t, expected, li.MonthlyUSD, 1e-3)
}

// --- GCP Cloud DNS ---

func TestDNSZone_CloudDNS_ZonesOnly(t *testing.T) {
	resetRegistry(t)
	Register(dnsZoneEstimator{})
	e, _ := Get("dns.zone")

	stubDNSZoneLookup(t, func(_ context.Context, shard string, f catalog.DNSZoneFilter) ([]catalog.Row, error) {
		require.Equal(t, "gcp-cloud-dns", shard)
		require.Equal(t, "public", f.ResourceName)
		require.Equal(t, "global", f.Region)
		return []catalog.Row{{
			SKUID: "cloud-dns-zone", Provider: "gcp", Service: "cloud-dns",
			ResourceName: "public", Region: "global",
			Prices: []catalog.Price{
				{Dimension: "hosted_zone", Tier: "0", TierUpper: "25", Amount: 0.20, Unit: "mo"},
				{Dimension: "hosted_zone", Tier: "25", TierUpper: "", Amount: 0.10, Unit: "mo"},
			},
		}}, nil
	})

	item, err := ParseItem("gcp/cloud-dns:public:region=global:zones=3")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.20*3, li.MonthlyUSD, 1e-6)
	require.Equal(t, "cloud-dns-zone", li.SKUID)
}

func TestDNSZone_CloudDNS_QueriesOnly(t *testing.T) {
	resetRegistry(t)
	Register(dnsZoneEstimator{})
	e, _ := Get("dns.zone")

	stubDNSZoneLookup(t, func(_ context.Context, _ string, _ catalog.DNSZoneFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "cloud-dns-query", Provider: "gcp", Service: "cloud-dns",
			ResourceName: "public", Region: "global",
			Prices: []catalog.Price{
				{Dimension: "query", Tier: "0", TierUpper: "1B", Amount: 0.0000004, Unit: "query"},
				{Dimension: "query", Tier: "1B", TierUpper: "", Amount: 0.0000002, Unit: "query"},
			},
		}}, nil
	})

	item, err := ParseItem("gcp/cloud-dns:public:region=global:queries=500000000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 5e8*0.0000004, li.MonthlyUSD, 1e-3)
}

// --- Negative tests ---

func TestDNSZone_MissingZonesAndQueries(t *testing.T) {
	resetRegistry(t)
	Register(dnsZoneEstimator{})
	e, _ := Get("dns.zone")

	stubDNSZoneLookup(t, func(_ context.Context, _ string, _ catalog.DNSZoneFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "r53-public-global", Provider: "aws", Service: "route53",
			ResourceName: "public", Region: "global",
			Prices: []catalog.Price{
				{Dimension: "hosted_zone", Tier: "0", TierUpper: "", Amount: 0.50},
			},
		}}, nil
	})

	item, err := ParseItem("aws/route53:public:region=global")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "zones")
}

func TestDNSZone_UnknownShard(t *testing.T) {
	resetRegistry(t)
	Register(dnsZoneEstimator{})
	e, _ := Get("dns.zone")

	// Build Item directly — provider/service not in shard map.
	item := Item{
		Raw:      "azure/dns:public:region=global:zones=1",
		Provider: "azure", Service: "dns",
		Resource: "public", Kind: "dns.zone",
		Params: map[string]string{"region": "global", "zones": "1"},
	}
	_, err := e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no shard")
}

func TestDNSZone_NoRows(t *testing.T) {
	resetRegistry(t)
	Register(dnsZoneEstimator{})
	e, _ := Get("dns.zone")

	stubDNSZoneLookup(t, func(_ context.Context, _ string, _ catalog.DNSZoneFilter) ([]catalog.Row, error) {
		return nil, nil
	})

	item, err := ParseItem("aws/route53:public:region=global:zones=1")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
}
